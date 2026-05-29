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

// DeviceCodeResponse holds the response from a device code request.
type DeviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
}

// DeviceCodeTokenResponse holds the response from polling for a token.
type DeviceCodeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// PollDeviceCodeFlow implements RFC 8628 device code flow polling.
func PollDeviceCodeFlow(ctx context.Context, opts DeviceCodePollOptions) (Credentials, error) {
	interval := opts.Interval
	if interval <= 0 {
		interval = 5
	}

	deadline := time.Now().Add(time.Duration(opts.ExpiresIn) * time.Second)
	if opts.Deadline > 0 {
		deadline = time.Now().Add(opts.Deadline)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return Credentials{}, ctx.Err()
		default:
		}

		if time.Now().After(deadline) {
			return Credentials{}, fmt.Errorf("device code flow expired")
		}

		time.Sleep(time.Duration(interval) * time.Second)

		data := url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {opts.DeviceCode},
			"client_id":    {opts.ClientID},
		}

		if len(opts.Scopes) > 0 {
			data.Set("scope", strings.Join(opts.Scopes, " "))
		}

		req, err := http.NewRequestWithContext(ctx, "POST", opts.TokenURL, strings.NewReader(data.Encode()))
		if err != nil {
			return Credentials{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return Credentials{}, err
		}

		var tokenResp DeviceCodeTokenResponse
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil && resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return Credentials{}, fmt.Errorf("device code request failed with status %d", resp.StatusCode)
		}
		resp.Body.Close()

		switch tokenResp.Error {
		case "":
			// Success
			creds := Credentials{
				Access:  tokenResp.AccessToken,
				Refresh: tokenResp.RefreshToken,
			}
			if tokenResp.ExpiresIn > 0 {
				creds.Expires = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Unix()
			}
			return creds, nil

		case "slow_down":
			interval += 5
			if opts.OnProgress != nil {
				opts.OnProgress("Rate limited, slowing down...")
			}
		case "authorization_pending":
			// Continue polling
			if opts.OnProgress != nil {
				opts.OnProgress("Waiting for authorization...")
			}
		case "expired_token":
			return Credentials{}, fmt.Errorf("device code expired")
		case "access_denied":
			return Credentials{}, fmt.Errorf("access denied")
		default:
			return Credentials{}, fmt.Errorf("device code error: %s - %s", tokenResp.Error, tokenResp.ErrorDesc)
		}
	}
}

// DeviceCodePollOptions configures device code flow polling.
type DeviceCodePollOptions struct {
	TokenURL    string
	ClientID    string
	DeviceCode  string
	Scopes      []string
	Interval    int
	ExpiresIn   int
	Deadline    time.Duration
	OnProgress  func(string)
}
