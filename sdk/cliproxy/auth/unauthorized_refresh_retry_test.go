package auth

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/internal/registry"
	cliproxyexecutor "github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/executor"
)

type unauthorizedRefreshExecutor struct {
	executeCalls int32
	streamCalls  int32
	refreshCalls int32
}

func (e *unauthorizedRefreshExecutor) Identifier() string { return "codex" }

func (e *unauthorizedRefreshExecutor) Execute(_ context.Context, auth *Auth, _ cliproxyexecutor.Request, _ cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	call := atomic.AddInt32(&e.executeCalls, 1)
	if call == 1 {
		return cliproxyexecutor.Response{}, &Error{HTTPStatus: http.StatusUnauthorized, Message: "status 401: unauthorized"}
	}
	if got := testStringValue(auth.Metadata["access_token"]); got != "fresh-token" {
		return cliproxyexecutor.Response{}, &Error{HTTPStatus: http.StatusUnauthorized, Message: "stale token was reused"}
	}
	return cliproxyexecutor.Response{Payload: []byte("ok")}, nil
}

func (e *unauthorizedRefreshExecutor) ExecuteStream(_ context.Context, auth *Auth, _ cliproxyexecutor.Request, _ cliproxyexecutor.Options) (*cliproxyexecutor.StreamResult, error) {
	call := atomic.AddInt32(&e.streamCalls, 1)
	if call == 1 {
		return nil, &Error{HTTPStatus: http.StatusUnauthorized, Message: "status 401: unauthorized"}
	}
	if got := testStringValue(auth.Metadata["access_token"]); got != "fresh-token" {
		return nil, &Error{HTTPStatus: http.StatusUnauthorized, Message: "stale token was reused"}
	}
	ch := make(chan cliproxyexecutor.StreamChunk, 1)
	ch <- cliproxyexecutor.StreamChunk{Payload: []byte("data: ok\n\n")}
	close(ch)
	return &cliproxyexecutor.StreamResult{Chunks: ch}, nil
}

func (e *unauthorizedRefreshExecutor) Refresh(_ context.Context, auth *Auth) (*Auth, error) {
	atomic.AddInt32(&e.refreshCalls, 1)
	updated := auth.Clone()
	if updated.Metadata == nil {
		updated.Metadata = make(map[string]any)
	}
	updated.Metadata["access_token"] = "fresh-token"
	return updated, nil
}

func (e *unauthorizedRefreshExecutor) CountTokens(context.Context, *Auth, cliproxyexecutor.Request, cliproxyexecutor.Options) (cliproxyexecutor.Response, error) {
	return cliproxyexecutor.Response{}, &Error{HTTPStatus: http.StatusNotImplemented, Message: "count not implemented"}
}

func (e *unauthorizedRefreshExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, &Error{HTTPStatus: http.StatusNotImplemented, Message: "http not implemented"}
}

func TestManagerExecute_RefreshesAndRetriesOnceAfterUnauthorized(t *testing.T) {
	const model = "gpt-5.5"
	executor := &unauthorizedRefreshExecutor{}
	manager := NewManager(nil, nil, nil)
	manager.RegisterExecutor(executor)

	auth := &Auth{ID: "auth-unauthorized-refresh", Provider: "codex", Metadata: map[string]any{"access_token": "stale-token"}}
	if _, err := manager.Register(WithSkipPersist(context.Background()), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}
	registry.GetGlobalRegistry().RegisterClient(auth.ID, "codex", []*registry.ModelInfo{{ID: model}})
	t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(auth.ID) })

	resp, err := manager.Execute(context.Background(), []string{"codex"}, cliproxyexecutor.Request{Model: model}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if string(resp.Payload) != "ok" {
		t.Fatalf("payload = %q, want ok", string(resp.Payload))
	}
	if got := atomic.LoadInt32(&executor.executeCalls); got != 2 {
		t.Fatalf("execute calls = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&executor.refreshCalls); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
	current, ok := manager.GetByID(auth.ID)
	if !ok {
		t.Fatal("expected auth in manager")
	}
	if current.Failed != 0 {
		t.Fatalf("failed count = %d, want 0", current.Failed)
	}
	if current.Success != 1 {
		t.Fatalf("success count = %d, want 1", current.Success)
	}
	if got := testStringValue(current.Metadata["access_token"]); got != "fresh-token" {
		t.Fatalf("stored access token = %q, want fresh-token", got)
	}
}

func TestManagerExecuteStream_RefreshesAndRetriesOnceAfterUnauthorized(t *testing.T) {
	const model = "gpt-5.5"
	executor := &unauthorizedRefreshExecutor{}
	manager := NewManager(nil, nil, nil)
	manager.RegisterExecutor(executor)

	auth := &Auth{ID: "auth-stream-unauthorized-refresh", Provider: "codex", Metadata: map[string]any{"access_token": "stale-token"}}
	if _, err := manager.Register(WithSkipPersist(context.Background()), auth); err != nil {
		t.Fatalf("register auth: %v", err)
	}
	registry.GetGlobalRegistry().RegisterClient(auth.ID, "codex", []*registry.ModelInfo{{ID: model}})
	t.Cleanup(func() { registry.GetGlobalRegistry().UnregisterClient(auth.ID) })

	result, err := manager.ExecuteStream(context.Background(), []string{"codex"}, cliproxyexecutor.Request{Model: model}, cliproxyexecutor.Options{})
	if err != nil {
		t.Fatalf("ExecuteStream error: %v", err)
	}
	if result == nil || result.Chunks == nil {
		t.Fatal("expected stream result")
	}
	chunk, ok := <-result.Chunks
	if !ok {
		t.Fatal("stream closed before payload")
	}
	if string(chunk.Payload) != "data: ok\n\n" {
		t.Fatalf("stream payload = %q, want data: ok", string(chunk.Payload))
	}
	for range result.Chunks {
	}
	if got := atomic.LoadInt32(&executor.streamCalls); got != 2 {
		t.Fatalf("stream calls = %d, want 2", got)
	}
	if got := atomic.LoadInt32(&executor.refreshCalls); got != 1 {
		t.Fatalf("refresh calls = %d, want 1", got)
	}
	current, ok := manager.GetByID(auth.ID)
	if !ok {
		t.Fatal("expected auth in manager")
	}
	if current.Failed != 0 {
		t.Fatalf("failed count = %d, want 0", current.Failed)
	}
	if current.Success != 1 {
		t.Fatalf("success count = %d, want 1", current.Success)
	}
}
