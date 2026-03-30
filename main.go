package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/amer/aql/internal/agent"
	"github.com/amer/aql/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if err := agent.CheckEnv(os.Getenv("ANTHROPIC_API_KEY")); err != nil {
		return err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load saved model or use default
	savedModel, err := agent.LoadModel(workDir)
	if err != nil {
		slog.Warn("failed to load saved model", "error", err)
	}
	if savedModel == "" {
		savedModel = string(agent.ResolveModel(""))
	}

	coder, err := agent.New(agent.Config{
		Name:         "coder",
		Role:         "Senior Go developer",
		SystemPrompt: "You are a senior Go developer. Be concise and helpful.",
		Model:        savedModel,
	}, workDir)
	if err != nil {
		return err
	}

	// Fetch available models from API
	ctx := context.Background()
	apiModels, err := agent.FetchModels(ctx)
	if err != nil {
		slog.Warn("failed to fetch models from API", "error", err)
	}

	var modelOptions []tui.ModelOption
	for _, m := range apiModels {
		modelOptions = append(modelOptions, tui.ModelOption{
			ID:             m.ID,
			DisplayName:    m.DisplayName,
			MaxInputTokens: m.MaxInputTokens,
		})
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
	model.SetAvailableModels(modelOptions)

	// Handle model selection persistence
	model.SetOnModelSelected(func(modelID string) {
		if err := agent.SaveModel(workDir, modelID); err != nil {
			slog.Error("failed to save model selection", "error", err)
		} else {
			slog.Info("model selection saved", "model", modelID)
		}
	})

	program = tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
