package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/auth"
	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/tui"
)

const (
	// bashCommandTimeout is the timeout for user-initiated bash commands.
	bashCommandTimeout = 5 * time.Minute

	// modelProbeTimeout is the timeout for background model probing.
	modelProbeTimeout = 30 * time.Second

	// loginTimeout is the timeout for the OAuth login flow.
	loginTimeout = 2 * time.Minute
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

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if useOAuth {
		apiKey = tokens.APIKey
	}

	// Load saved model + cached model list for instant startup
	savedModel, err := agent.LoadModel(workDir)
	if err != nil {
		slog.Warn("failed to load saved model", "error", err)
	}

	cachedModels, _ := agent.LoadModelCache(workDir)
	if savedModel == "" && len(cachedModels) > 0 {
		savedModel = cachedModels[0].ID
		slog.Info("auto-selected model from cache", "model", savedModel)
	}
	if savedModel == "" {
		savedModel = string(agent.ResolveModel(""))
	}

	cfg := agent.Config{
		Name: "coder",
		Role: "Senior Go developer",
		SystemPrompt: `You are a senior Go developer working in an interactive terminal. Be concise and helpful.

Always use the most appropriate tool. Prefer edit over write_file for modifying existing files. Use glob to discover files before reading them. Use ask_user when requirements are ambiguous rather than guessing.`,
		Model: savedModel,
	}

	// program is declared early so askUser closure can capture it.
	// It is assigned after NewModel below.
	var program *tea.Program

	askUser := func(ctx context.Context, q agent.UserQuestion) (string, error) {
		responseCh := make(chan string, 1)
		program.Send(tui.AgentAskUserMsg{
			AgentName:  "coder",
			Question:   q.Question,
			ResponseCh: responseCh,
		})
		select {
		case answer := <-responseCh:
			return answer, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	opts := []agent.Option{agent.WithAskUser(askUser)}
	if useOAuth {
		opts = append(opts, agent.WithOAuthKey(tokens.APIKey))
	}

	coder, err := agent.New(cfg, workDir, opts...)
	if err != nil {
		return err
	}

	var streamCancel context.CancelFunc

	onSubmit := func(input string) tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			streamCancel = cancel

			// Show spinner immediately before the API call
			program.Send(tui.AgentStreamStartMsg{AgentName: "coder"})

			ch := coder.Run(ctx, input)

			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case evt, ok := <-ch:
						if !ok {
							return
						}
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
						if evt.ToolCall != nil {
							// ask_user is displayed via AgentAskUserMsg, not as a tool block
							if evt.ToolCall.ToolName != "ask_user" {
								program.Send(tui.AgentToolCallMsg{
									AgentName: evt.AgentName,
									ToolCall: domain.ToolCall{
										Name:    evt.ToolCall.ToolName,
										Content: evt.ToolCall.Input,
										Status:  domain.ToolRunning,
										ToolID:  evt.ToolCall.ToolID,
									},
								})
							}
							continue
						}
						if evt.ToolDone != nil {
							// ask_user results are already shown inline
							if evt.ToolDone.ToolName == "ask_user" {
								continue
							}
							status := domain.ToolDone
							if evt.ToolDone.IsError {
								status = domain.ToolError
							}
							program.Send(tui.AgentToolCallMsg{
								AgentName: evt.AgentName,
								ToolCall: domain.ToolCall{
									Name:    evt.ToolDone.ToolName,
									Content: evt.ToolDone.Output,
									Status:  status,
									ToolID:  evt.ToolDone.ToolID,
								},
							})
							continue
						}
						if evt.TokenUsage != nil {
							program.Send(tui.TokenUsageMsg{
								InputTokens:  evt.TokenUsage.InputTokens,
								OutputTokens: evt.TokenUsage.OutputTokens,
							})
							continue
						}
						if evt.Text != "" {
							program.Send(tui.AgentStreamDeltaMsg{
								AgentName: evt.AgentName,
								Delta:     evt.Text,
							})
						}
					}
				}
			}()

			return nil
		}
	}

	onBash := func(command string) tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), bashCommandTimeout)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", command)
			cmd.Dir = workDir
			out, err := cmd.CombinedOutput()
			return tui.BashResultMsg{
				Command: command,
				Output:  string(out),
				Error:   err,
			}
		}
	}

	model := tui.NewModel("aql", []string{"coder"}, onSubmit)
	model.SetProjectPath(workDir)
	model.SetModelName(savedModel)
	model.SetOnBash(onBash)

	// Use cached models for instant startup
	if len(cachedModels) > 0 {
		model.SetModelTiers(modelsToTiers(cachedModels))
	}
	model.SetOnClear(func() {
		coder.ClearHistory()
	})
	model.SetOnCompact(func() tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			summary, err := coder.CompactHistory(ctx)
			return tui.CompactDoneMsg{Summary: summary, Err: err}
		}
	})
	model.SetCancelStream(func() {
		if streamCancel != nil {
			streamCancel()
		}
	})
	model.SetOnModelSelected(func(modelID string) {
		if err := agent.SaveModel(workDir, modelID); err != nil {
			slog.Error("failed to save model selection", "error", err)
			return
		}
		slog.Info("model selection saved", "model", modelID)

		cfg.Model = modelID
		newCoder, createErr := agent.New(cfg, workDir, opts...)
		if createErr != nil {
			slog.Error("failed to recreate agent with new model", "model", modelID, "error", createErr)
			return
		}
		coder = newCoder
		slog.Info("agent recreated with new model", "model", modelID)
	})

	// Mouse tracking enabled for scroll wheel chat scrolling.
	// Hold Shift to select and copy text (standard terminal behavior).
	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Cancel background work when the TUI exits
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()

	// Probe models in background — updates TUI and cache when done
	go func() {
		ctx, cancel := context.WithTimeout(bgCtx, modelProbeTimeout)
		defer cancel()

		var usableModels []domain.ModelInfo
		var probeErr error
		if useOAuth {
			usableModels, probeErr = agent.ProbeUsableModelsWithOAuthKey(ctx, apiKey)
		} else {
			usableModels, probeErr = agent.ProbeUsableModelsWithAPIKey(ctx, apiKey)
		}
		if probeErr != nil {
			slog.Warn("background model probe failed", "error", probeErr)
			return
		}
		if len(usableModels) == 0 {
			return
		}

		// Update cache
		if err := agent.SaveModelCache(workDir, usableModels); err != nil {
			slog.Warn("failed to save model cache", "error", err)
		}

		// Update TUI model list
		program.Send(tui.ModelsLoadedMsg{Tiers: modelsToTiers(usableModels)})

		// If saved model isn't in the usable list, auto-select best
		if savedModel != "" && !isModelUsable(savedModel, usableModels) {
			slog.Warn("saved model not accessible, switching to best available",
				"saved", savedModel, "usable", len(usableModels))
		}
	}()

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

	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
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

func isModelUsable(modelID string, usable []domain.ModelInfo) bool {
	for _, m := range usable {
		if m.ID == modelID {
			return true
		}
	}
	return false
}

func modelsToTiers(models []domain.ModelInfo) []tui.ModelTier {
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
