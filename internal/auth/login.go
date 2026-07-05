package auth

// ──────────────────────────────────────────────────────────────────
// FILE GUIDELINES
//
// BELONGS HERE:
//   - Login() — full OAuth PKCE flow
//   - startCallbackServer — local HTTP server for OAuth callback
//   - exchangeAndCreateKey, openAuthURL, openBrowser
//   - LoginResult/LoginOptions types
//   - Success HTML page, generateState
//
// MUST NOT GO HERE:
//   - Token storage (oauth.go's SaveTokens/LoadTokens)
//   - API key resolution (resolve.go)
//   - Agent or TUI imports
//
// Q: Should I change the OAuth flow?
// A: Modify Login() here. It orchestrates: server start → browser
//    open → wait for code → exchange → create key.
// ──────────────────────────────────────────────────────────────────

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// LoginResult contains the result of a login flow.
type LoginResult struct {
	Tokens *Tokens
	Error  error
}

// LoginOptions configures the OAuth login flow.
type LoginOptions struct {
	// Console uses Anthropic Console (API billing) instead of Claude.ai subscription.
	Console bool
	// TokenURL overrides the token endpoint (for testing).
	TokenURL string
	// OpenURL is called to open the authorization URL. Defaults to opening a browser.
	OpenURL func(url string) error
	// HTTPClient is the HTTP client used for token exchange and API key creation.
	// Defaults to http.DefaultClient if nil.
	HTTPClient *http.Client
}

// callbackServer holds the state for the local OAuth callback HTTP server.
type callbackServer struct {
	server *http.Server
	codeCh chan string
	errCh  chan error
	port   int
}

// startCallbackServer creates and starts a local HTTP server that listens for
// the OAuth callback with the authorization code.
func startCallbackServer(expectedState string) (*callbackServer, error) {
	// Bind the listener up front and hand it directly to Serve. This avoids a
	// TOCTOU window (another process grabbing the port between probe and bind)
	// and removes the need to sleep waiting for the server to come up — the
	// socket is already listening before Serve is called.
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("listen on callback port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		gotState := r.URL.Query().Get("state")
		if gotState != expectedState {
			errCh <- fmt.Errorf("state mismatch: expected %q, got %q", expectedState, gotState)
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		if code == "" {
			errMsg := r.URL.Query().Get("error")
			if errMsg == "" {
				errMsg = "no code in callback"
			}
			errCh <- fmt.Errorf("authorization failed: %s", errMsg)
			http.Error(w, "Authorization failed", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, successPage)
		codeCh <- code
	})

	srv := &http.Server{Handler: mux}

	go func() {
		slog.Debug("starting OAuth callback server", "port", port)
		if err := srv.Serve(listener); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()

	return &callbackServer{server: srv, codeCh: codeCh, errCh: errCh, port: port}, nil
}

func (cs *callbackServer) shutdown() {
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cs.server.Shutdown(shutdownCtx)
}

// openAuthURL opens the authorization URL in the user's browser.
func openAuthURL(authURL string, opener func(string) error) {
	if opener == nil {
		opener = openBrowser
	}
	if err := opener(authURL); err != nil {
		slog.Warn("failed to open browser", "error", err)
		fmt.Printf("\nOpen this URL in your browser to login:\n%s\n\n", authURL)
	}
}

// exchangeAndCreateKey exchanges the authorization code for tokens, then creates an API key.
func exchangeAndCreateKey(client *http.Client, tokenURL, code, verifier, state string, port int) (*Tokens, error) {
	slog.Debug("received authorization code")
	tokens, err := ExchangeCode(client, tokenURL, code, verifier, state, port)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}

	// Create an API key from the OAuth token — the Messages API requires
	// a real API key, not the OAuth Bearer token directly.
	slog.Debug("creating API key from OAuth token")
	apiKey, err := CreateAPIKey(client, CreateAPIKeyURL, tokens.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}
	tokens.APIKey = apiKey
	// SECURITY: never log the API key or a prefix of it.
	slog.Info("login successful", "hasAPIKey", apiKey != "")
	return tokens, nil
}

// Login runs the full OAuth PKCE login flow:
// 1. Starts a local callback server
// 2. Opens the browser to the authorization URL
// 3. Waits for the callback with the authorization code
// 4. Exchanges the code for tokens
func Login(ctx context.Context, opts LoginOptions) (*Tokens, error) {
	tokenURL := opts.TokenURL
	if tokenURL == "" {
		tokenURL = DefaultTokenURL
	}

	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	verifier, challenge := GeneratePKCE()
	state, err := generateState()
	if err != nil {
		return nil, err
	}

	cs, err := startCallbackServer(state)
	if err != nil {
		return nil, err
	}
	defer cs.shutdown()

	authURL := BuildAuthorizeURL(challenge, state, cs.port, opts.Console)
	slog.Info("opening browser for login", "url", authURL, "console", opts.Console)
	openAuthURL(authURL, opts.OpenURL)

	select {
	case code := <-cs.codeCh:
		return exchangeAndCreateKey(client, tokenURL, code, verifier, state, cs.port)
	case err := <-cs.errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

const successPage = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>AQL — Login Successful</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Orbitron:wght@700;900&family=Inter:wght@400;500&display=swap" rel="stylesheet">
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    background: #1f2335;
    font-family: 'Inter', -apple-system, BlinkMacSystemFont, sans-serif;
    color: #a9b1d6;
  }
  .card {
    text-align: center;
    padding: 3.5rem 4.5rem;
    background: #24283b;
    border: 1px solid #545c7e;
    border-radius: 16px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    max-width: 500px;
  }
  .logo {
    font-family: 'Orbitron', monospace;
    font-weight: 900;
    font-size: 4rem;
    letter-spacing: 0.15em;
    color: #7aa2f7;
    text-shadow: 0 0 30px rgba(122, 162, 247, 0.3);
    margin-bottom: 2rem;
  }
  .icon {
    width: 56px;
    height: 56px;
    margin: 0 auto 1.5rem;
    border-radius: 50%;
    background: rgba(158, 206, 106, 0.12);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .icon svg { width: 28px; height: 28px; }
  h1 {
    font-size: 1.4rem;
    font-weight: 500;
    color: #c0caf5;
    margin-bottom: 0.5rem;
  }
  .tagline {
    color: #545c7e;
    font-size: 0.85rem;
    letter-spacing: 0.08em;
    text-transform: uppercase;
    margin-bottom: 2rem;
  }
  .divider {
    width: 40px;
    height: 2px;
    background: #3d59a1;
    border-radius: 1px;
    margin: 1.25rem auto;
  }
  p {
    color: #737aa2;
    font-size: 0.95rem;
    line-height: 1.6;
  }
  .inspire {
    color: #ff9e64;
    font-size: 0.9rem;
    font-style: italic;
    margin-top: 1.5rem;
  }
  @keyframes check-in {
    0% { transform: scale(0.5); opacity: 0; }
    60% { transform: scale(1.15); }
    100% { transform: scale(1); opacity: 1; }
  }
  @keyframes glow {
    0%, 100% { text-shadow: 0 0 30px rgba(122, 162, 247, 0.3); }
    50% { text-shadow: 0 0 40px rgba(122, 162, 247, 0.5), 0 0 60px rgba(122, 162, 247, 0.15); }
  }
  .icon { animation: check-in 0.5s ease-out; }
  .logo { animation: glow 3s ease-in-out infinite; }
</style>
</head>
<body>
  <div class="card">
    <div class="logo">AQL</div>
    <div class="tagline">Agent Quorum Loop</div>
    <div class="icon">
      <svg viewBox="0 0 24 24" fill="none" stroke="#9ece6a" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="20 6 9 17 4 12"/>
      </svg>
    </div>
    <h1>Login successful!</h1>
    <div class="divider"></div>
    <p>You can close this tab and return to the terminal.</p>
    <p class="inspire">Now go build something great.</p>
  </div>
</body>
</html>`

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate CSRF state: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
