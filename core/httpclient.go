package core

import (
	"net/http"
	"time"
)

var (
	SSEClient = &http.Client{
		Timeout: 0,
		Transport: &http.Transport{
			ResponseHeaderTimeout: 2 * time.Minute,
			IdleConnTimeout:       30 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
		},
	}

	RegularClient = &http.Client{
		Timeout: 30 * time.Second,
	}
)

// RequestClient returns the HTTP client to use for a streaming (SSE) request.
// If opts.Fetch is set, it is used; otherwise the shared SSEClient is returned.
// This lets callers inject a custom client for observability, recording, custom
// TLS, or proxies without replacing the default. Callers MUST NOT mutate the
// returned shared client (SSEClient) per request.
// || 返回流式（SSE）请求使用的 HTTP client。若 opts.Fetch 已设置则用它，
// || 否则返回共享的 SSEClient。调用方可注入自定义 client 以做观测/录制/
// || 自定义 TLS/代理。切勿按请求修改返回的共享 client（SSEClient）。
func RequestClient(opts StreamOptions) *http.Client {
	if opts.Fetch != nil {
		return opts.Fetch
	}
	return SSEClient
}
