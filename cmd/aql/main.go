package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/auth"
	"github.com/amer/aql/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Redirect logs to file so they don't corrupt the TUI
	logFile, err := os.OpenFile("aql.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFile.Close()
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(logFile)

	// Handle `aql auth login` subcommand
	if len(os.Args) > 2 && os.Args[1] == "auth" && os.Args[2] == "login" {
		return runLogin()
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Try OAuth tokens first, fall back to API key
	tokens, tokenErr := auth.LoadTokens(workDir)
	slog.Debug("OAuth token check", "workDir", workDir, "found", tokens != nil, "error", tokenErr)
	if tokens == nil {
		if homeDir, err := os.UserHomeDir(); err == nil {
			tokens, _ = auth.LoadTokens(homeDir)
		}
	}

	var useOAuth bool
	if tokens != nil && !tokens.IsExpired() {
		useOAuth = true
		slog.Info("using OAuth authentication", "expiresAt", tokens.ExpiresAt)
	} else if tokens != nil && tokens.IsExpired() {
		slog.Warn("OAuth tokens expired, falling back to API key")
	}

	if !useOAuth {
		if err := agent.CheckEnv(os.Getenv("ANTHROPIC_API_KEY")); err != nil {
			return fmt.Errorf("%w\n\n  Or run: aql auth login --console", err)
		}
	}

	// Probe which models the API key can actually use
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if useOAuth {
		apiKey = tokens.APIKey
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	var usableModels []agent.ModelInfo
	var probeErr error
	if useOAuth {
		usableModels, probeErr = agent.ProbeUsableModelsWithOAuthKey(ctx, apiKey)
	} else {
		usableModels, probeErr = agent.ProbeUsableModelsWithAPIKey(ctx, apiKey)
	}
	cancel()
	if probeErr != nil {
		slog.Warn("failed to probe models", "error", probeErr)
	}

	// Load saved model, validate against usable models
	savedModel, err := agent.LoadModel(workDir)
	if err != nil {
		slog.Warn("failed to load saved model", "error", err)
	}

	// If saved model isn't usable, pick the best usable one
	if savedModel != "" && !isModelUsable(savedModel, usableModels) {
		slog.Warn("saved model not accessible, switching to best available",
			"saved", savedModel, "usable", len(usableModels))
		savedModel = ""
	}
	if savedModel == "" && len(usableModels) > 0 {
		savedModel = usableModels[0].ID // Already sorted newest first
		slog.Info("auto-selected model", "model", savedModel)
	}
	if savedModel == "" {
		savedModel = string(agent.ResolveModel(""))
	}

	cfg := agent.Config{
		Name:         "coder",
		Role:         "Senior Go developer",
		SystemPrompt: "You are a senior Go developer. Be concise and helpful.",
		Model:        savedModel,
	}

	var coder *agent.Agent
	if useOAuth {
		coder, err = agent.NewWithOAuthKey(cfg, workDir, tokens.APIKey)
	} else {
		coder, err = agent.New(cfg, workDir)
	}
	if err != nil {
		return err
	}

	var program *tea.Program

	onSubmit := func(input string) tea.Cmd {
		return func() tea.Msg {
			ctx := context.Background()
			ch := coder.Run(ctx, input)

			go func() {
				for evt := range ch {
					if evt.Error != nil {
						program.Send(tui.AgentStreamErrorMsg{
							AgentName: evt.AgentName,
							Error:     evt.Error,
						})
						return
					}
					if evt.Done {
						program.Send(tui.AgentStreamDoneMsg{
							AgentName: evt.AgentName,
						})
						return
					}
					program.Send(tui.AgentStreamDeltaMsg{
						AgentName: evt.AgentName,
						Delta:     evt.Text,
					})
				}
			}()

			return nil
		}
	}

	model := tui.NewModel("aql", []string{"coder"}, onSubmit)
	model.SetProjectPath(workDir)
	model.SetModelName(savedModel)

	// Set dynamic model tiers from probed usable models
	if len(usableModels) > 0 {
		model.SetModelTiers(modelsToTiers(usableModels))
	}
	model.SetOnModelSelected(func(modelID string) {
		if err := agent.SaveModel(workDir, modelID); err != nil {
			slog.Error("failed to save model selection", "error", err)
			return
		}
		slog.Info("model selection saved", "model", modelID)

		cfg.Model = modelID
		var newCoder *agent.Agent
		var createErr error
		if useOAuth {
			newCoder, createErr = agent.NewWithOAuthKey(cfg, workDir, tokens.APIKey)
		} else {
			newCoder, createErr = agent.New(cfg, workDir)
		}
		if createErr != nil {
			slog.Error("failed to recreate agent with new model", "model", modelID, "error", createErr)
			return
		}
		coder = newCoder
		slog.Info("agent recreated with new model", "model", modelID)
	})

	program = tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func runLogin() error {
	console := false
	for _, arg := range os.Args[3:] {
		if arg == "--console" {
			console = true
		}
	}

	fmt.Println("Logging in to Anthropic...")
	if console {
		fmt.Println("Using Console (API billing) login")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tokens, err := auth.Login(ctx, auth.LoginOptions{Console: console})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := auth.SaveTokens(workDir, *tokens); err != nil {
		return fmt.Errorf("save tokens: %w", err)
	}

	fmt.Printf("Login successful! Tokens saved to %s/.aql_tokens.json\n", workDir)
	fmt.Printf("Token expires at: %s\n", tokens.ExpiresAt.Format("2006-01-02 15:04:05"))
	return nil
}

func isModelUsable(modelID string, usable []agent.ModelInfo) bool {
	for _, m := range usable {
		if m.ID == modelID {
			return true
		}
	}
	return false
}

func modelsToTiers(models []agent.ModelInfo) []tui.ModelTier {
	tiers := make([]tui.ModelTier, len(models))
	for i, m := range models {
		ctx := fmt.Sprintf("%dk context", m.MaxInputTokens/1000)
		tiers[i] = tui.ModelTier{
			Label:       m.DisplayName,
			ModelID:     m.ID,
			Description: ctx,
		}
	}
	return tiers
}
