package tui

import (
	"fmt"
	"strings"
)

// WelcomeData holds the data needed to render the welcome screen.
type WelcomeData struct {
	AppName     string
	Version     string
	ProjectPath string
	HomeDir     string
	ModelName   string
	Username    string
	Width       int
}

// WelcomeGreeting returns a personalized or generic greeting.
func WelcomeGreeting(username string) string {
	if username != "" && len(username) <= 20 {
		return fmt.Sprintf("Welcome back %s!", username)
	}
	return "Welcome back!"
}

// ShortenHome replaces a home directory prefix with ~.
func ShortenHome(path, home string) string {
	if home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

// TruncatePath shortens a filesystem path to fit within maxWidth,
// replacing middle segments with "…". Keeps first and last segments.
func TruncatePath(path string, maxWidth int) string {
	if len(path) <= maxWidth {
		return path
	}

	parts := strings.Split(path, "/")
	if len(parts) <= 2 {
		return path
	}

	first := parts[0]
	last := parts[len(parts)-1]

	middle := parts[1 : len(parts)-1]
	kept := []string{}

	for i := len(middle) - 1; i >= 0; i-- {
		candidate := append([]string{middle[i]}, kept...)
		trial := first + "/…/" + strings.Join(candidate, "/") + "/" + last
		if len(trial) > maxWidth {
			break
		}
		kept = candidate
	}

	if len(kept) == 0 {
		return first + "/…/" + last
	}
	return first + "/…/" + strings.Join(kept, "/") + "/" + last
}

// RenderLogo returns a stylized "A" in block characters.
func RenderLogo() string {
	lines := []string{
		" ▄█▄ ",
		"█▀ ▀█",
		"█▄▄▄█",
		"█   █",
	}
	return strings.Join(lines, "\n")
}

// RenderWelcome renders a compact welcome banner that scrolls with content.
func RenderWelcome(data WelcomeData) string {
	greeting := WelcomeGreeting(data.Username)
	path := ShortenHome(data.ProjectPath, data.HomeDir)
	path = TruncatePath(path, 40)

	line1 := WelcomeGreetStyle.Render(greeting)
	line2 := WelcomeDimStyle.Render(path + "  ·  " + data.ModelName)

	return line1 + "\n" + line2
}
