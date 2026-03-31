package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - FormatTokenCount — comma-separated formatting,
//     FormatTokenCountShort — short form (1.0k, 1.2m).
//
// MUST NOT GO HERE:
//   - Token tracking state, API pricing logic, status bar rendering
//     (statusbar.go).
// ──────────────────────────────────────────────────────────────────

import "fmt"

// FormatTokenCount formats a token count with comma separators.
func FormatTokenCount(n int) string {
	if n < 0 {
		return "-" + FormatTokenCount(-n)
	}
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}

	// Build from right to left with comma groups
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// FormatTokenCountShort formats a token count in short form (1.0k, 1.2m).
func FormatTokenCountShort(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fm", float64(n)/1_000_000)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
