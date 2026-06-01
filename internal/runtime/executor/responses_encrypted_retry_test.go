package executor

import (
	"net/http"
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

func TestIsInvalidResponsesEncryptedContentError(t *testing.T) {
	body := []byte(`{
		"error":{
			"code":"invalid_encrypted_content",
			"type":"invalid_request_error",
			"message":"The encrypted content gAAA...Vw== could not be verified. Reason: Encrypted content could not be decrypted or parsed."
		}
	}`)

	if !isInvalidResponsesEncryptedContentError(http.StatusBadRequest, body) {
		t.Fatalf("expected invalid encrypted content error to be detected")
	}
	if isInvalidResponsesEncryptedContentError(http.StatusInternalServerError, body) {
		t.Fatalf("non-400 response should not trigger encrypted content fallback")
	}
}

func TestShouldRetryResponsesWithoutEncryptedReasoningDoesNotRetryContextTooLarge(t *testing.T) {
	body := []byte(`{
		"error":{
			"code":"context_too_large",
			"type":"invalid_request_error",
			"message":"Your input exceeds the context window of this model. Please adjust your input and try again."
		}
	}`)

	if shouldRetryResponsesWithoutEncryptedReasoning(http.StatusBadRequest, body) {
		t.Fatalf("context_too_large should be surfaced to the client, not retried by CPA")
	}
	if shouldRetryResponsesWithoutEncryptedReasoning(http.StatusRequestEntityTooLarge, body) {
		t.Fatalf("413 context length response should be surfaced to the client, not retried by CPA")
	}
	if shouldRetryResponsesWithoutEncryptedReasoning(http.StatusInternalServerError, body) {
		t.Fatalf("non-client context response should not trigger encrypted reasoning fallback")
	}
}

func TestStripInvalidEncryptedContentFromResponsesBody(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"message","role":"user","content":"hello"},
			{"type":"reasoning","id":"rs_bad","encrypted_content":"gAAA"},
			{"type":"function_call","call_id":"call_123","name":"lookup","arguments":"{}"},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done","encrypted_content":"nested"}]}
		]
	}`)

	got, changed := stripInvalidEncryptedContentFromResponsesBody(raw)
	if !changed {
		t.Fatalf("expected body to be changed")
	}
	items := gjson.GetBytes(got, "input").Array()
	if len(items) != 3 {
		t.Fatalf("expected reasoning item to be removed, got %d items: %s", len(items), got)
	}
	if typ := gjson.GetBytes(got, "input.0.type").String(); typ != "message" {
		t.Fatalf("first input should remain message, got %q; body=%s", typ, got)
	}
	if typ := gjson.GetBytes(got, "input.1.type").String(); typ != "function_call" {
		t.Fatalf("function call should remain, got %q; body=%s", typ, got)
	}
	if strings.Contains(string(got), "encrypted_content") {
		t.Fatalf("encrypted_content should be removed from retry body: %s", got)
	}
}

func TestStripReasoningContextForRetryRemovesReasoningWithoutEncryptedContent(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"reasoning","id":"rs_sanitized","summary":[]},
			{"type":"message","role":"user","content":"hello"}
		]
	}`)
	errBody := []byte(`{"error":{"code":"context_too_large","message":"Your input exceeds the context window of this model."}}`)

	got, changed := stripReasoningContextForRetry(raw, errBody)
	if !changed {
		t.Fatalf("expected body to be changed")
	}
	items := gjson.GetBytes(got, "input").Array()
	if len(items) != 1 {
		t.Fatalf("expected reasoning item to be removed, got %d items: %s", len(items), got)
	}
	if typ := gjson.GetBytes(got, "input.0.type").String(); typ != "message" {
		t.Fatalf("message input should remain, got %q; body=%s", typ, got)
	}
}

func TestStripReasoningContextForRetryAcceptsStreamFailedEvent(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.4",
		"input":[
			{"type":"reasoning","id":"rs_sanitized","summary":[]},
			{"type":"message","role":"user","content":"hello"}
		]
	}`)
	errBody := []byte(`{"type":"response.failed","response":{"error":{"code":"context_too_large","message":"Your input exceeds the context window of this model."}}}`)

	got, changed := stripReasoningContextForRetry(raw, errBody)
	if !changed {
		t.Fatalf("expected body to be changed")
	}
	if typ := gjson.GetBytes(got, "input.0.type").String(); typ != "message" {
		t.Fatalf("message input should remain, got %q; body=%s", typ, got)
	}
}
