package main

import (
	"context"
	"fmt"
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

	coder, err := agent.New(agent.Config{
		Name:         "coder",
		Role:         "Senior Go developer",
		SystemPrompt: "You are a senior Go developer. Be concise and helpful.",
	}, workDir)
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

	resolvedModel := string(agent.ResolveModel(""))
	model := tui.NewModel("aql", []string{"coder"}, onSubmit)
	model.SetProjectPath(workDir)
	model.SetModelName(resolvedModel)

	program = tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
