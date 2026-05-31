package executor

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type codexRoundTripFunc func(*http.Request) (*http.Response, error)

func (f codexRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDoCodexRequestWithHeaderTimeoutCancelsBeforeHeaders(t *testing.T) {
	client := &http.Client{Transport: codexRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.invalid", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext error: %v", err)
	}

	started := time.Now()
	_, err = doCodexRequestWithHeaderTimeout(client, req, 10*time.Millisecond)
	if err == nil {
		t.Fatal("expected header timeout error")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("header timeout took too long: %s", elapsed)
	}
	if !strings.Contains(err.Error(), "codex response header timeout") {
		t.Fatalf("error = %v, want codex response header timeout", err)
	}
}

func TestDoCodexRequestWithHeaderTimeoutDoesNotCancelBodyAfterHeaders(t *testing.T) {
	client := &http.Client{Transport: codexRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		pr, pw := io.Pipe()
		go func() {
			time.Sleep(30 * time.Millisecond)
			_, _ = pw.Write([]byte("ok"))
			_ = pw.Close()
		}()
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       pr,
			Header:     make(http.Header),
			Request:    req,
		}, nil
	})}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.invalid", nil)
	if err != nil {
		t.Fatalf("NewRequestWithContext error: %v", err)
	}

	resp, err := doCodexRequestWithHeaderTimeout(client, req, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("Do error: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll error: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", string(body))
	}
}
