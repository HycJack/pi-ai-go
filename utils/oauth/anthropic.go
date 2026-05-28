package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	anthropicAuthURL  = "https://console.anthropic.com/oauth/authorize"
	anthropicTokenURL = "https://console.anthropic.com/oauth/token"
	anthropicPort     = 53692
	// Client ID is base64 encoded
	anthropicClientIDB64 = "ZGNfZWY0OGNhMzktNjZhYi00YTIwLWFhYjktOTMxYmI0ZGI1ZTY5"
)

func getAnthropicClientID() string {
	b, _ := base64.StdEncoding.DecodeString(anthropicClientIDB64)
	return string(b)
}

func loginAnthropic(ctx context.Context, callbacks LoginCallbacks) (Credentials, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/callback", anthropicPort)

	state, err := generateState()
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to generate state: %w", err)
	}

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256",
		anthropicAuthURL,
		url.QueryEscape(getAnthropicClientID()),
		url.QueryEscape(redirectURI),
		url.QueryEscape("org:create_api_key user:profile user:inference user:sessions:claude_code user:mcp_servers user:file_upload"),
		url.QueryEscape(state),
		url.QueryEscape(pkce.Challenge),
	)

	if callbacks.OnAuth != nil {
		callbacks.OnAuth(authURL)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}

		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("state mismatch")
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(oauthSuccessHTML("Anthropic")))
		codeCh <- code
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", anthropicPort),
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	defer server.Shutdown(context.Background())

	select {
	case <-ctx.Done():
		return Credentials{}, ctx.Err()
	case err := <-errCh:
		return Credentials{}, err
	case code := <-codeCh:
		return exchangeAnthropicCode(ctx, code, pkce.Verifier, redirectURI)
	}
}

func exchangeAnthropicCode(ctx context.Context, code, verifier, redirectURI string) (Credentials, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {getAnthropicClientID()},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return Credentials{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return Credentials{}, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return Credentials{}, err
	}

	creds := Credentials{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
	}
	if tokenResp.ExpiresIn > 0 {
		creds.Expires = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
	}
	return creds, nil
}

func refreshAnthropic(ctx context.Context, credentials Credentials) (Credentials, error) {
	if credentials.Refresh == "" {
		return credentials, fmt.Errorf("no refresh token available")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {credentials.Refresh},
		"client_id":     {getAnthropicClientID()},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", anthropicTokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return credentials, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return credentials, err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return credentials, err
	}

	creds := Credentials{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
	}
	if tokenResp.ExpiresIn > 0 {
		creds.Expires = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
	}
	return creds, nil
}

func generateState() (string, error) {
	b := make([]byte, 16)
	if _, err := time.Now().MarshalBinary(); err == nil {
		// Use time as seed isn't great, but we also use crypto/rand
	}
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func oauthSuccessHTML(provider string) string {
	return `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>Login Successful</title>
<style>body{background:#1a1a2e;color:#e0e0e0;font-family:system-ui;display:flex;justify-content:center;align-items:center;height:100vh;margin:0}
.container{text-align:center}.check{font-size:64px;color:#4ade80}</style></head>
<body><div class="container"><div class="check">✓</div>
<h1>Login Successful</h1><p>You may close this window.</p></div></body></html>`
}
