package oauth

import (
	"context"
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
	codexAuthURL  = "https://auth0.openai.com/authorize"
	codexTokenURL = "https://auth0.openai.com/oauth/token"
	codexPort     = 1455
	codexClientID = "DRivsnmYLKgM1v3oVGGcN9rgCoGWxjCs"
)

func loginOpenAICodex(ctx context.Context, callbacks LoginCallbacks) (Credentials, error) {
	pkce, err := GeneratePKCE()
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to generate PKCE: %w", err)
	}

	redirectURI := fmt.Sprintf("http://localhost:%d/callback", codexPort)

	state, err := generateState()
	if err != nil {
		return Credentials{}, fmt.Errorf("failed to generate state: %w", err)
	}

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&scope=%s&state=%s&code_challenge=%s&code_challenge_method=S256&audience=https://api.openai.com/v1&prompt=login",
		codexAuthURL,
		url.QueryEscape(codexClientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape("openid profile email"),
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
		w.Write([]byte(oauthSuccessHTML("OpenAI Codex")))
		codeCh <- code
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", codexPort),
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
		return exchangeCodexCode(ctx, code, pkce.Verifier, redirectURI)
	}
}

func exchangeCodexCode(ctx context.Context, code, verifier, redirectURI string) (Credentials, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {codexClientID},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", codexTokenURL, strings.NewReader(data.Encode()))
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
		IDToken      string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return Credentials{}, err
	}

	creds := Credentials{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
		Extra:   make(map[string]any),
	}
	if tokenResp.ExpiresIn > 0 {
		creds.Expires = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
	}

	// Extract account ID from JWT
	if tokenResp.IDToken != "" {
		if accountID := extractJWTAccountID(tokenResp.IDToken); accountID != "" {
			creds.Extra["accountId"] = accountID
		}
	}

	return creds, nil
}

func refreshOpenAICodex(ctx context.Context, credentials Credentials) (Credentials, error) {
	if credentials.Refresh == "" {
		return credentials, fmt.Errorf("no refresh token available")
	}

	client := &http.Client{Timeout: 30 * time.Second}

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {credentials.Refresh},
		"client_id":     {codexClientID},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", codexTokenURL, strings.NewReader(data.Encode()))
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
		IDToken      string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return credentials, err
	}

	creds := Credentials{
		Access:  tokenResp.AccessToken,
		Refresh: tokenResp.RefreshToken,
		Extra:   credentials.Extra,
	}
	if tokenResp.ExpiresIn > 0 {
		creds.Expires = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
	}

	if tokenResp.IDToken != "" {
		if accountID := extractJWTAccountID(tokenResp.IDToken); accountID != "" {
			if creds.Extra == nil {
				creds.Extra = make(map[string]any)
			}
			creds.Extra["accountId"] = accountID
		}
	}

	return creds, nil
}

func extractJWTAccountID(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}

	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}
