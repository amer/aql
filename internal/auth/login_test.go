package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amer/aql/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestLoginFlow_EndToEnd(t *testing.T) {
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		json.NewDecoder(r.Body).Decode(&req)

		assert.Equal(t, "authorization_code", req["grant_type"])
		assert.NotEmpty(t, req["code"])
		assert.NotEmpty(t, req["code_verifier"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "test-access-token-from-login",
			"refresh_token": "test-refresh-token-from-login",
			"expires_in":    3600,
		})
	}))
	defer tokenServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	urlCh := make(chan string, 1)

	resultCh := make(chan *auth.LoginResult, 1)
	go func() {
		tokens, err := auth.Login(ctx, auth.LoginOptions{
			Console:  true,
			TokenURL: tokenServer.URL,
			OpenURL: func(url string) error {
				urlCh <- url
				return nil
			},
		})
		resultCh <- &auth.LoginResult{Tokens: tokens, Error: err}
	}()

	var capturedURL string
	select {
	case capturedURL = <-urlCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OpenURL callback")
	}

	assert.Contains(t, capturedURL, "platform.claude.com/oauth/authorize")
	assert.Contains(t, capturedURL, "scope=")
	assert.Contains(t, capturedURL, "user:inference")
	assert.Contains(t, capturedURL, "org:create_api_key")

	cancel()

	result := <-resultCh
	assert.Error(t, result.Error)
}

func TestLoginFlow_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err := auth.Login(ctx, auth.LoginOptions{
		TokenURL: "http://localhost:1/unused",
		OpenURL:  func(string) error { return nil },
	})

	assert.ErrorIs(t, err, context.DeadlineExceeded,
		"should return context error when no callback arrives")
}

func TestBuildAuthorizeURL_Components(t *testing.T) {
	_, challenge := auth.GeneratePKCE()
	url := auth.BuildAuthorizeURL(challenge, "test-state", 49152, false)
	assert.Contains(t, url, "49152")
	assert.Contains(t, url, "user:inference")
}
