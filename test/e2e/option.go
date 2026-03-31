package e2e

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - config — private struct holding Terminal constructor settings,
//     defaultConfig() — sensible defaults for terminal dimensions,
//     Option — functional option type for Terminal,
//     WithBinary() — use a pre-built binary,
//     WithSize() — set terminal dimensions,
//     WithWorkDir() — set process working directory,
//     WithEnv() — add environment variables,
//     WithRecordAPI() — enable API call recording.
//
// MUST NOT GO HERE:
//   - Terminal implementation (terminal.go)
//   - Recorder implementation (recorder.go)
//   - Screenshot types (screenshot.go)
//
// Q: Should I add a new Terminal configuration option?
// A: Yes, add a With*() function here that modifies the config struct.
//
// Q: Should I add a new config field?
// A: Add the field to the config struct here, set its default in
//    defaultConfig(), and expose it via a With*() option.
// ──────────────────────────────────────────────────────────────────

// config holds Terminal constructor settings.
type config struct {
	binary    string   // path to pre-built binary (empty = build fresh)
	cols      int      // terminal width
	rows      int      // terminal height
	workDir   string   // working directory for the process
	env       []string // additional environment variables
	recordAPI bool     // start a recording proxy for API calls
}

func defaultConfig() config {
	return config{
		cols: 120,
		rows: 40,
	}
}

// Option configures a Terminal.
type Option func(*config)

// WithBinary uses a pre-built binary instead of building fresh.
func WithBinary(path string) Option {
	return func(c *config) { c.binary = path }
}

// WithSize sets terminal dimensions.
func WithSize(cols, rows int) Option {
	return func(c *config) {
		c.cols = cols
		c.rows = rows
	}
}

// WithWorkDir sets the working directory for the spawned process.
func WithWorkDir(dir string) Option {
	return func(c *config) { c.workDir = dir }
}

// WithEnv adds environment variables (KEY=VALUE format).
func WithEnv(env ...string) Option {
	return func(c *config) { c.env = append(c.env, env...) }
}

// WithRecordAPI enables API call recording via a local reverse proxy.
// Captured exchanges are saved to the artifacts directory on cleanup.
func WithRecordAPI() Option {
	return func(c *config) { c.recordAPI = true }
}
