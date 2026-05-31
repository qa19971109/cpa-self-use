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

func TestShouldRetryResponsesWithoutEncryptedReasoningForContextTooLarge(t *testing.T) {
	body := []byte(`{
		"error":{
			"code":"context_too_large",
			"type":"invalid_request_error",
			"message":"Your input exceeds the context window of this model. Please adjust your input and try again."
		}
	}`)

	if !shouldRetryResponsesWithoutEncryptedReasoning(http.StatusBadRequest, body) {
		t.Fatalf("expected context_too_large to trigger encrypted reasoning fallback")
	}
	if !shouldRetryResponsesWithoutEncryptedReasoning(http.StatusRequestEntityTooLarge, body) {
		t.Fatalf("expected 413 context length response to trigger encrypted reasoning fallback")
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

func TestBuildTextFileHistoryContextFallbackForRetry(t *testing.T) {
	raw := []byte(`{
		"model":"gpt-5.5",
		"previous_response_id":"resp_old",
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"第一轮用户问题"}]},
			{"type":"message","role":"assistant","content":[{"type":"output_text","text":"第一轮回答"}]},
			{"type":"reasoning","id":"rs_1","encrypted_content":"gAAA"},
			{"type":"function_call","call_id":"call_1","name":"read_file","arguments":"{\"path\":\"a.txt\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"文件内容"},
			{"type":"message","role":"user","content":[{"type":"input_text","text":"最后请修复这个问题"}]}
		]
	}`)
	errBody := []byte(`{"error":{"code":"context_too_large","message":"Your input exceeds the context window of this model."}}`)

	got, changed := buildTextFileHistoryContextFallbackForRetry(raw, errBody)
	if !changed {
		t.Fatalf("expected fallback body to be built")
	}
	if gjson.GetBytes(got, "previous_response_id").Exists() {
		t.Fatalf("previous_response_id should be removed: %s", got)
	}
	if gjson.GetBytes(got, "include").Exists() {
		t.Fatalf("include should be removed from text file fallback: %s", got)
	}
	if typ := gjson.GetBytes(got, "input.0.content.1.type").String(); typ != "input_file" {
		t.Fatalf("fallback should attach history as input_file, got %q: %s", typ, got)
	}
	if filename := gjson.GetBytes(got, "input.0.content.1.filename").String(); filename != "history.txt" {
		t.Fatalf("fallback filename = %q, want history.txt; body=%s", filename, got)
	}
	fileData := gjson.GetBytes(got, "input.0.content.1.file_data").String()
	if !strings.HasPrefix(fileData, "data:text/plain;base64,") {
		t.Fatalf("fallback file_data should be text/plain data URI: %s", fileData)
	}
	input := gjson.GetBytes(got, "input").Raw
	for _, want := range []string{"history.txt", "用户最后一条要求", "最后请修复这个问题"} {
		if !strings.Contains(input, want) {
			t.Fatalf("fallback input missing %q: %s", want, input)
		}
	}
	if strings.Contains(input, "encrypted_content") || strings.Contains(input, "gAAA") {
		t.Fatalf("fallback input should not include encrypted reasoning payload: %s", input)
	}
}

func TestBuildTextFileHistoryContextFallbackForRetryIgnoresOtherErrors(t *testing.T) {
	raw := []byte(`{"model":"gpt-5.5","input":"hello"}`)
	errBody := []byte(`{"error":{"code":"rate_limit_exceeded","message":"slow down"}}`)

	if _, changed := buildTextFileHistoryContextFallbackForRetry(raw, errBody); changed {
		t.Fatalf("non-context error should not build text fallback")
	}
}
