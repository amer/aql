package main

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - The main() and run() functions — program entrypoint
//   - Wiring: creating agents, LLM clients, TUI, and connecting them via callbacks
//   - CLI subcommand routing (e.g. `auth login`)
//   - Logging setup
//   - Background goroutines for model probing
//   - Callback closures that bridge TUI <-> Agent (onSubmit, onBash, onClear, etc.)
//
// MUST NOT GO HERE:
//   - Business logic or domain rules — those belong in internal/agent or internal/domain
//   - Tool implementations — those belong in internal/agent/tools
//   - TUI rendering or styling — those belong in internal/tui
//   - Auth logic beyond routing — that belongs in internal/auth
//   - Direct Anthropic SDK calls — those belong in internal/llm
//
// Q: Should I add a new CLI flag here?
// A: Yes, parse it in run() and thread it to the appropriate package.
//
// Q: Should I add a new callback for the TUI?
// A: Yes, define it here as a closure that captures the agent, then inject via model.Set*().
//
// Q: Should I handle a new message type here?
// A: No, message handling goes in internal/tui/handlers.go. Main only sends messages via program.Send().
//
// Q: Where do I add a new tool?
// A: Not here. Add it in internal/agent/tools/defs.go (definition + handler + display mapping).
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/agent/tools"
	"github.com/amer/aql/internal/auth"
	"github.com/amer/aql/internal/diff"
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

	// Single HTTP client for all outbound network calls.
	httpClient := &http.Client{Transport: http.DefaultTransport}

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
	chatClient := llm.NewAnthropicClient(llm.WithAPIKey(apiKey), llm.WithHTTPClient(httpClient))

	// Options shared by the primary agent and every spawned sub-agent,
	// so children inherit OAuth billing and any future base configuration.
	baseOpts := []agent.Option{agent.WithChatClient(chatClient)}
	if useOAuth {
		baseOpts = append(baseOpts, agent.WithOAuth())
	}

	// Build tool executor with the shared HTTP client
	spawner := agent.NewSpawner(chatClient, cfg, workDir, agent.WithAgentOptions(baseOpts...))
	toolExec := tools.NewExecutor(
		tools.WithTaskStore(tools.NewTaskStore()),
		tools.WithAgentSpawner(spawner),
		tools.WithAskUser(askUser),
		tools.WithHTTPClient(httpClient),
	)

	opts := append(slices.Clone(baseOpts),
		agent.WithAskUser(askUser),
		agent.WithToolExecutor(toolExec),
	)

	coder, err := agent.New(cfg, workDir, opts...)
	if err != nil {
		return err
	}

	var streamCancel context.CancelFunc

	model := configureTUI(cfg, workDir, cachedModels, coder, &streamCancel, &program)

	program = tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	probeCfg := models.ClientConfig{APIKey: apiKey, WithBilling: useOAuth, HTTPClient: httpClient}
	startBackgroundModelProbe(probeCfg, workDir, program)

	_, err = program.Run()
	return err
}

func setupLogging() error {
	// 0600: the log may contain sensitive request/response metadata; keep it owner-only.
	logFile, err := os.OpenFile("aql.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
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
) tui.Model {
	onSubmit := func(input string) tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithCancel(context.Background())
			*streamCancel = cancel
			(*program).Send(tui.AgentStreamStartMsg{AgentName: "coder"})
			ch := coder.Run(ctx, input)
			go stream.ForwardWithHistory(ctx, ch,
				func(msg any) { (*program).Send(msg) },
				stream.HistoryCallbacks{
					Append:  func(msg domain.Message) { coder.ApplyHistory(msg) },
					Replace: func(msgs []domain.Message) { coder.ReplaceHistory(msgs) },
				},
			)
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
	diffRunner := diff.NewDefaultRunner()
	model.SetOnDiff(func() tea.Cmd {
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			files, stats, err := diffRunner.Run(ctx, workDir)
			return tui.DiffResultMsg{Files: files, Stats: stats, Err: err}
		}
	})
	model.SetOnModelSelected(func(modelID string) {
		if err := models.SaveModel(workDir, modelID); err != nil {
			slog.Error("failed to save model selection", "error", err)
			return
		}
		// Mutate only the model on the live agent. Recreating it and copying
		// the struct over *coder would copy the agent's mutex and race the
		// history slice with any in-flight Run goroutine.
		coder.SetModel(modelID)
	})

	return model
}

func startBackgroundModelProbe(cfg models.ClientConfig, workDir string, program *tea.Program) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), modelProbeTimeout)
		defer cancel()
		models.ProbeAndUpdate(ctx, cfg, workDir, func(usable []domain.ModelInfo) {
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
