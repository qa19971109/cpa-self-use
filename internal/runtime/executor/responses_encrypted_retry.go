package executor

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
)

const contextFallbackHistoryCharLimit = 120000

func isInvalidResponsesEncryptedContentError(statusCode int, body []byte) bool {
	if statusCode != http.StatusBadRequest {
		return false
	}
	for _, path := range []string{"error.code", "detail.code", "code"} {
		if strings.EqualFold(strings.TrimSpace(gjson.GetBytes(body, path).String()), "invalid_encrypted_content") {
			return true
		}
	}

	msgParts := []string{
		gjson.GetBytes(body, "error.message").String(),
		gjson.GetBytes(body, "detail").String(),
		string(body),
	}
	for _, msg := range msgParts {
		msg = strings.ToLower(msg)
		if strings.Contains(msg, "invalid_encrypted_content") {
			return true
		}
		if strings.Contains(msg, "encrypted content") &&
			(strings.Contains(msg, "could not be verified") || strings.Contains(msg, "could not be decrypted")) {
			return true
		}
	}
	return false
}

func shouldRetryResponsesWithoutEncryptedReasoning(statusCode int, body []byte) bool {
	if isInvalidResponsesEncryptedContentError(statusCode, body) {
		return true
	}
	if statusCode != http.StatusBadRequest && statusCode != http.StatusRequestEntityTooLarge {
		return false
	}
	return codexTerminalErrorIsContextLength(body)
}

func stripInvalidEncryptedContentFromResponsesBody(body []byte) ([]byte, bool) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil || root == nil {
		return body, false
	}
	input, ok := root["input"]
	if !ok {
		return body, false
	}
	strippedInput, changed, keep := stripInvalidEncryptedContentValue(input, false)
	if !changed {
		return body, false
	}
	if keep {
		root["input"] = strippedInput
	} else {
		delete(root, "input")
	}
	stripped, err := json.Marshal(root)
	if err != nil {
		return body, false
	}
	return stripped, true
}

func stripReasoningItemsFromResponsesBody(body []byte) ([]byte, bool) {
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil || root == nil {
		return body, false
	}
	input, ok := root["input"]
	if !ok {
		return body, false
	}
	strippedInput, changed, keep := stripReasoningItemsValue(input, false)
	if !changed {
		return body, false
	}
	if keep {
		root["input"] = strippedInput
	} else {
		delete(root, "input")
	}
	stripped, err := json.Marshal(root)
	if err != nil {
		return body, false
	}
	return stripped, true
}

func stripReasoningContextForRetry(requestBody, errorBody []byte) ([]byte, bool) {
	isContextLength := codexTerminalErrorIsContextLength(errorBody)
	if !isContextLength {
		_, isContextLength = codexTerminalStreamContextLengthErr(errorBody)
	}
	if isContextLength {
		if stripped, changed := stripReasoningItemsFromResponsesBody(requestBody); changed {
			return stripped, true
		}
	}
	return stripInvalidEncryptedContentFromResponsesBody(requestBody)
}

func buildTextFileHistoryContextFallbackForRetry(requestBody, errorBody []byte) ([]byte, bool) {
	isContextLength := codexTerminalErrorIsContextLength(errorBody)
	if !isContextLength {
		_, isContextLength = codexTerminalStreamContextLengthErr(errorBody)
	}
	if !isContextLength {
		return requestBody, false
	}

	var root map[string]any
	if err := json.Unmarshal(requestBody, &root); err != nil || root == nil {
		return requestBody, false
	}
	input, ok := root["input"]
	if !ok {
		return requestBody, false
	}

	history, lastRequest := splitResponsesInputHistoryAndLastRequest(input)
	history = trimKeepTail(history, contextFallbackHistoryCharLimit)
	lastRequest = strings.TrimSpace(lastRequest)
	if history == "" && lastRequest == "" {
		return requestBody, false
	}

	var b strings.Builder
	b.WriteString("以下内容是历史会话记录，可能由原始结构化上下文压缩而来。请把它当作背景参考，不要逐字复述。")
	if history != "" {
		b.WriteString("\n\n<历史会话记录>\n")
		b.WriteString(history)
		b.WriteString("\n</历史会话记录>")
	}
	historyFileText := b.String()
	fileData := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(historyFileText))

	instructionText := "已附上 history.txt，它是历史会话记录，请把它当作背景参考。请优先完成下面的用户最后一条要求。"
	lastRequestText := lastRequest
	if lastRequestText == "" {
		lastRequestText = "请根据 history.txt 继续处理用户请求。"
	}

	content := []any{
		map[string]any{
			"type": "input_text",
			"text": instructionText,
		},
		map[string]any{
			"type":      "input_file",
			"filename":  "history.txt",
			"file_data": fileData,
		},
	}
	if lastRequest != "" {
		content = append(content, map[string]any{
			"type": "input_text",
			"text": "用户最后一条要求:\n" + lastRequestText,
		})
	}

	root["input"] = []any{
		map[string]any{
			"type":    "message",
			"role":    "user",
			"content": content,
		},
	}
	delete(root, "previous_response_id")
	delete(root, "include")
	stripped, err := json.Marshal(root)
	if err != nil {
		return requestBody, false
	}
	return stripped, true
}

func stripInvalidEncryptedContentValue(value any, arrayItem bool) (any, bool, bool) {
	switch v := value.(type) {
	case []any:
		changed := false
		out := make([]any, 0, len(v))
		for _, item := range v {
			stripped, itemChanged, keep := stripInvalidEncryptedContentValue(item, true)
			if itemChanged {
				changed = true
			}
			if !keep {
				changed = true
				continue
			}
			out = append(out, stripped)
		}
		return out, changed, true
	case map[string]any:
		changed := false
		if strings.TrimSpace(firstNonEmptyAnyString(v["type"])) == "reasoning" {
			if _, hasEncrypted := v["encrypted_content"]; hasEncrypted {
				if arrayItem {
					return nil, true, false
				}
				delete(v, "encrypted_content")
				changed = true
			}
		} else if _, hasEncrypted := v["encrypted_content"]; hasEncrypted {
			delete(v, "encrypted_content")
			changed = true
		}
		for key, child := range v {
			stripped, childChanged, keep := stripInvalidEncryptedContentValue(child, false)
			if childChanged {
				changed = true
			}
			if keep {
				v[key] = stripped
			} else {
				delete(v, key)
			}
		}
		return v, changed, true
	default:
		return value, false, true
	}
}

func stripReasoningItemsValue(value any, arrayItem bool) (any, bool, bool) {
	switch v := value.(type) {
	case []any:
		changed := false
		out := make([]any, 0, len(v))
		for _, item := range v {
			stripped, itemChanged, keep := stripReasoningItemsValue(item, true)
			if itemChanged {
				changed = true
			}
			if !keep {
				changed = true
				continue
			}
			out = append(out, stripped)
		}
		return out, changed, true
	case map[string]any:
		if strings.TrimSpace(firstNonEmptyAnyString(v["type"])) == "reasoning" && arrayItem {
			return nil, true, false
		}
		changed := false
		for key, child := range v {
			stripped, childChanged, keep := stripReasoningItemsValue(child, false)
			if childChanged {
				changed = true
			}
			if keep {
				v[key] = stripped
			} else {
				delete(v, key)
			}
		}
		return v, changed, true
	default:
		return value, false, true
	}
}

func firstNonEmptyAnyString(values ...any) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func splitResponsesInputHistoryAndLastRequest(input any) (string, string) {
	items, ok := input.([]any)
	if !ok {
		return "", responseInputText(input)
	}
	if len(items) == 0 {
		return "", ""
	}

	lastIdx := len(items) - 1
	for i := len(items) - 1; i >= 0; i-- {
		if responseInputRole(items[i]) == "user" {
			lastIdx = i
			break
		}
	}

	var history strings.Builder
	for i, item := range items {
		text := strings.TrimSpace(responseInputText(item))
		if text == "" {
			continue
		}
		if i == lastIdx {
			continue
		}
		role := responseInputRole(item)
		if role == "" {
			role = responseInputType(item)
		}
		if role == "" {
			role = "item"
		}
		if history.Len() > 0 {
			history.WriteString("\n\n")
		}
		history.WriteString("[")
		history.WriteString(role)
		history.WriteString("]\n")
		history.WriteString(text)
	}
	return history.String(), responseInputText(items[lastIdx])
}

func responseInputRole(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(firstNonEmptyAnyString(m["role"])))
}

func responseInputType(value any) string {
	m, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(firstNonEmptyAnyString(m["type"])))
}

func responseInputText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, item := range v {
			text := strings.TrimSpace(responseInputText(item))
			if text == "" {
				continue
			}
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(text)
		}
		return b.String()
	case map[string]any:
		typ := strings.ToLower(strings.TrimSpace(firstNonEmptyAnyString(v["type"])))
		switch typ {
		case "reasoning":
			return strings.TrimSpace(responseInputText(v["summary"]))
		case "message":
			return responseInputText(v["content"])
		case "function_call":
			name := firstNonEmptyAnyString(v["name"])
			args := responseInputText(v["arguments"])
			if name == "" {
				return args
			}
			if args == "" {
				return "工具调用: " + name
			}
			return "工具调用: " + name + "\n参数: " + args
		case "function_call_output":
			out := responseInputText(v["output"])
			if out == "" {
				out = responseInputText(v["content"])
			}
			if out == "" {
				return ""
			}
			return "工具结果:\n" + out
		case "input_text", "output_text", "text":
			return firstNonEmptyAnyString(v["text"], v["content"])
		case "input_image", "image_url":
			return "[图片内容已省略]"
		}
		if text := firstNonEmptyAnyString(v["text"], v["content"], v["output"], v["arguments"]); text != "" {
			return text
		}
		return ""
	default:
		return ""
	}
}

func trimKeepTail(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return "[历史会话前半部分因上下文过长已省略]\n" + s[len(s)-limit:]
}
