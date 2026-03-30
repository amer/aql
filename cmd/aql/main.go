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

	onSubmit := func(input string) tea.Cmd {
		return func() tea.Msg {
			ctx := context.Background()
			ch := coder.Run(ctx, input)

			// Collect first delta to kick off streaming
			for evt := range ch {
				if evt.Error != nil {
					return tui.AgentStreamErrorMsg{
						AgentName: evt.AgentName,
						Error:     evt.Error,
					}
				}
				if evt.Done {
					return tui.AgentStreamDoneMsg{
						AgentName: evt.AgentName,
					}
				}
				return tui.AgentStreamDeltaMsg{
					AgentName: evt.AgentName,
					Delta:     evt.Text,
				}
			}
			return nil
		}
	}

	// We need a way to keep reading from the channel after the first delta.
	// Use a different approach: send all events as messages via the program.
	var program *tea.Program

	onSubmitWithProgram := func(input string) tea.Cmd {
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
	_ = onSubmit // unused, using onSubmitWithProgram instead

	model := tui.NewModel("aql", []string{"coder"}, onSubmitWithProgram)

	program = tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}
