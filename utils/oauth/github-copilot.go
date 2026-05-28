package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	githubDeviceCodeURL = "https://github.com/login/device/code"
	githubTokenURL      = "https://github.com/login/oauth/access_token"
	copilotTokenURL     = "https://api.github.com/copilot_internal/v2/token"
	githubClientID      = "Iv1.b507a08c87ecfe98"
)

func loginGitHubCopilot(ctx context.Context, callbacks LoginCallbacks) (Credentials, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Request device code
	data := url.Values{
		"client_id": {githubClientID},
		"scope":     {"read:user"},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", githubDeviceCodeURL, strings.NewReader(data.Encode()))
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

	var deviceResp DeviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceResp); err != nil {
		return Credentials{}, err
	}

	if callbacks.OnDeviceCode != nil {
		callbacks.OnDeviceCode(deviceResp.UserCode, deviceResp.VerificationURI)
	}

	if callbacks.OnPrompt != nil {
		callbacks.OnPrompt(fmt.Sprintf("Go to %s and enter code: %s", deviceResp.VerificationURI, deviceResp.UserCode))
	}

	// Poll for token
	creds, err := PollDeviceCodeFlow(ctx, DeviceCodePollOptions{
		TokenURL:   githubTokenURL,
		ClientID:   githubClientID,
		DeviceCode: deviceResp.DeviceCode,
		Interval:   deviceResp.Interval,
		ExpiresIn:  deviceResp.ExpiresIn,
		OnProgress: callbacks.OnProgress,
	})
	if err != nil {
		return Credentials{}, err
	}

	// Exchange GitHub token for Copilot token
	copilotToken, err := getCopilotToken(ctx, creds.Access)
	if err != nil {
		return Credentials{}, err
	}

	creds.Access = copilotToken
	return creds, nil
}

func getCopilotToken(ctx context.Context, githubToken string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", copilotTokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.Token, nil
}

func refreshGitHubCopilot(ctx context.Context, credentials Credentials) (Credentials, error) {
	// GitHub Copilot tokens are short-lived, need to re-fetch from the GitHub token
	if credentials.Refresh == "" {
		return credentials, fmt.Errorf("no refresh token available")
	}

	// Use the refresh token (which is the GitHub token) to get a new Copilot token
	copilotToken, err := getCopilotToken(ctx, credentials.Refresh)
	if err != nil {
		return credentials, err
	}

	credentials.Access = copilotToken
	credentials.Expires = time.Now().Add(30 * time.Minute).Unix()
	return credentials, nil
}
