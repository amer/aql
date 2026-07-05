package tui

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - truncateEnd() — shorten to N runes, mark elision with ", +more",
//     truncateTail() — keep the last N runes behind a leading ellipsis.
//
// MUST NOT GO HERE:
//   - Styling/color (styles.go), block layout (transcript.go, diff.go).
//   - Anything that measures rendered width including ANSI escapes.
//
// Q: Why rune slices, not byte slices?
// A: Byte slicing panics or splits multibyte characters mid-rune. These
//    helpers count and cut on rune boundaries so any input is safe.
//
// Q: Do these account for double-width (CJK/emoji) display cells?
// A: No — they count runes, not terminal cells. Good enough for the
//    single-line summaries here; use a width library if that changes.
// ──────────────────────────────────────────────────────────────────

// truncateEnd shortens s to at most maxLen runes. When s overflows and there
// is room for it, the elision is marked with a trailing ", +more". Returns ""
// for a non-positive maxLen. Never slices past a rune boundary.
func truncateEnd(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	const suffix = ", +more"
	if maxLen <= len(suffix) {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-len(suffix)]) + suffix
}

// truncateTail keeps the last maxLen runes of s behind a leading "…" when s is
// longer than maxLen. Returns "" for a non-positive maxLen. Rune-safe.
func truncateTail(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen == 1 {
		return "…"
	}
	return "…" + string(r[len(r)-(maxLen-1):])
}
