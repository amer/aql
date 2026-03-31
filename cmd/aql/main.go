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
	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/auth"
	"github.com/amer/aql/internal/domain"
	"github.com/amer/aql/internal/llm"
	"github.com/amer/aql/internal/models"
	"github.com/amer/aql/internal/stream"
	"github.com/amer/aql/internal/tui"
)

const (
	bashCommandTimeout = 5 * time.Minute
	modelProbeTimeout  = 30 * time.Second
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := setupLogging(); err != nil {
		return err
	}

	// Handle `aql auth login` subcommand
	if len(os.Args) > 2 && os.Args[1] == "auth" && os.Args[2] == "login" {
		return auth.RunLoginCLI(os.Args[3:])
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	apiKey, useOAuth, err := auth.ResolveAPIKey(workDir)
	if err != nil {
		return err
	}

	savedModel, cachedModels := models.LoadOrDefault(workDir)

	cfg := agent.Config{
		Name: "coder",
		Role: "Senior Go developer",
		SystemPrompt: `You are a senior Go developer working in an interactive terminal. Be concise and helpful.

Always use the most appropriate tool. Prefer edit over write_file for modifying existing files. Use glob to discover files before reading them. Use ask_user when requirements are ambiguous rather than guessing.`,
		Model: savedModel,
	}

	var program *tea.Program

	askUser := func(ctx context.Context, q tools.UserQuestion) (string, error) {
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

	// Build the LLM adapter — OAuth-derived keys are still API keys
	// (sent via X-Api-Key header), the OAuth flag only controls billing headers
	chatClient := llm.NewAnthropicClient(llm.WithAPIKey(apiKey))

	opts := []agent.Option{
		agent.WithChatClient(chatClient),
		agent.WithAskUser(askUser),
	}
	if useOAuth {
		opts = append(opts, agent.WithOAuth())
	}

	coder, err := agent.New(cfg, workDir, opts...)
	if err != nil {
		return err
	}

	var streamCancel context.CancelFunc

	model := configureTUI(cfg, workDir, cachedModels, coder, &streamCancel, &program, opts)

	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	startBackgroundModelProbe(apiKey, useOAuth, workDir, program)

	_, err = program.Run()
	return err
}

func setupLogging() error {
	logFile, err := os.OpenFile("aql.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	// Note: logFile is intentionally not closed here — it stays open for the
	// lifetime of the process and is cleaned up on exit.
	slog.SetDefault(slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug})))
	log.SetOutput(logFile)
	return nil
}

func configureTUI(
	cfg agent.Config,
	workDir string,
	cachedModels []domain.ModelInfo,
	coder *agent.Agent,
	streamCancel *context.CancelFunc,
	program **tea.Program,
	opts []agent.Option,
) tui.Model {
	onSubmit := func(input string) tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			*streamCancel = cancel
			(*program).Send(tui.AgentStreamStartMsg{AgentName: "coder"})
			ch := coder.Run(ctx, input)
			go stream.Forward(ctx, ch, func(msg any) { (*program).Send(msg) })
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
			return tui.BashResultMsg{Command: command, Output: string(out), Error: err}
		}
	}

	model := tui.NewModel("aql", []string{"coder"}, onSubmit)
	model.SetProjectPath(workDir)
	model.SetModelName(cfg.Model)
	model.SetOnBash(onBash)
	if len(cachedModels) > 0 {
		model.SetModelTiers(modelsToTiers(cachedModels))
	}
	model.SetOnClear(func() { coder.ClearHistory() })
	model.SetOnCompact(func() tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			summary, err := coder.CompactHistory(ctx)
			return tui.CompactDoneMsg{Summary: summary, Err: err}
		}
	})
	model.SetCancelStream(func() {
		if *streamCancel != nil {
			(*streamCancel)()
		}
	})
	model.SetOnModelSelected(func(modelID string) {
		if err := models.SaveModel(workDir, modelID); err != nil {
			slog.Error("failed to save model selection", "error", err)
			return
		}
		cfg.Model = modelID
		newCoder, createErr := agent.New(cfg, workDir, opts...)
		if createErr != nil {
			slog.Error("failed to recreate agent with new model", "model", modelID, "error", createErr)
			return
		}
		*coder = *newCoder
		slog.Info("model switched", "model", modelID)
	})

	return model
}

func startBackgroundModelProbe(apiKey string, useOAuth bool, workDir string, program *tea.Program) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), modelProbeTimeout)
		defer cancel()
		models.ProbeAndUpdate(ctx, apiKey, useOAuth, workDir, func(usable []domain.ModelInfo) {
			program.Send(tui.ModelsLoadedMsg{Tiers: modelsToTiers(usable)})
		})
	}()
}

func modelsToTiers(ms []domain.ModelInfo) []tui.ModelTier {
	tiers := make([]tui.ModelTier, len(ms))
	for i, m := range ms {
		tiers[i] = tui.ModelTier{
			Label:       m.DisplayName,
			ModelID:     m.ID,
			Description: fmt.Sprintf("%dk context", m.MaxInputTokens/1000),
		}
	}
	return tiers
}
