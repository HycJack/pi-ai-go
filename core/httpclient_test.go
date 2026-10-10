package core

import (
	"net/http"
	"testing"
)

func TestRequestClientDefaultsToSSE(t *testing.T) {
	got := RequestClient(StreamOptions{})
	if got == nil {
		t.Fatal("RequestClient with empty options must return a non-nil client")
	}
	if got != SSEClient {
		t.Errorf("empty options should fall back to SSEClient, got %p", got)
	}
}

func TestRequestClientUsesFetch(t *testing.T) {
	custom := &http.Client{}
	got := RequestClient(StreamOptions{Fetch: custom})
	if got != custom {
		t.Errorf("RequestClient should return the Fetch client, got %p", got)
	}
}