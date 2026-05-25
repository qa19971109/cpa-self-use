package management

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	apihandlers "github.com/router-for-me/CLIProxyAPI/v7/sdk/api/handlers"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/cliproxy/auth"
	"github.com/tidwall/gjson"
)

const (
	defaultAuthFileTestModel  = "gpt-5.5"
	defaultAuthFileTestPrompt = "Reply exactly with: CPA_AUTH_TEST_OK"
)

type authFileTestRequest struct {
	AuthIndexSnake string `json:"auth_index"`
	AuthIndexCamel string `json:"authIndex"`
	Name           string `json:"name"`
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
}

type authFileTestResponse struct {
	OK          bool   `json:"ok"`
	AuthID      string `json:"auth_id,omitempty"`
	AuthIndex   string `json:"auth_index,omitempty"`
	Name        string `json:"name,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Model       string `json:"model,omitempty"`
	StatusCode  int    `json:"status_code,omitempty"`
	LatencyMS   int64  `json:"latency_ms"`
	Text        string `json:"text,omitempty"`
	RawResponse string `json:"raw_response,omitempty"`
	Error       string `json:"error,omitempty"`
}

// TestAuthFile sends a minimal non-streaming OpenAI Responses request pinned to one auth file.
func (h *Handler) TestAuthFile(c *gin.Context) {
	var body authFileTestRequest
	if errBindJSON := c.ShouldBindJSON(&body); errBindJSON != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	authIndex := strings.TrimSpace(body.AuthIndexSnake)
	if authIndex == "" {
		authIndex = strings.TrimSpace(body.AuthIndexCamel)
	}
	name := strings.TrimSpace(body.Name)
	if authIndex == "" && name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth_index or name is required"})
		return
	}

	selectedAuth := h.authByIndex(authIndex)
	if selectedAuth == nil {
		selectedAuth = h.authByName(name)
	}
	if selectedAuth == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "auth file not found"})
		return
	}
	if selectedAuth.Disabled || selectedAuth.Status == auth.StatusDisabled {
		selectedAuth.EnsureIndex()
		c.JSON(http.StatusOK, authFileTestResponse{
			OK:        false,
			AuthID:    selectedAuth.ID,
			AuthIndex: selectedAuth.Index,
			Name:      authFileName(selectedAuth),
			Provider:  strings.TrimSpace(selectedAuth.Provider),
			Model:     normalizeAuthFileTestModel(body.Model),
			Error:     "auth file is disabled",
		})
		return
	}

	h.mu.Lock()
	apiHandler := h.apiHandler
	h.mu.Unlock()
	if apiHandler == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "api handler not initialized"})
		return
	}

	model := normalizeAuthFileTestModel(body.Model)
	prompt := strings.TrimSpace(body.Prompt)
	if prompt == "" {
		prompt = defaultAuthFileTestPrompt
	}

	rawRequest, errMarshal := json.Marshal(map[string]any{
		"model":             model,
		"input":             prompt,
		"stream":            false,
		"max_output_tokens": 32,
	})
	if errMarshal != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build test request"})
		return
	}

	selectedAuth.EnsureIndex()
	startedAt := time.Now()
	execCtx := apihandlers.WithPinnedAuthID(c.Request.Context(), selectedAuth.ID)
	resp, _, errMsg := apiHandler.ExecuteWithAuthManager(execCtx, "openai-response", model, rawRequest, "")
	latencyMS := time.Since(startedAt).Milliseconds()

	out := authFileTestResponse{
		AuthID:    selectedAuth.ID,
		AuthIndex: selectedAuth.Index,
		Name:      authFileName(selectedAuth),
		Provider:  strings.TrimSpace(selectedAuth.Provider),
		Model:     model,
		LatencyMS: latencyMS,
	}
	if errMsg != nil {
		out.OK = false
		out.StatusCode = errMsg.StatusCode
		if errMsg.Error != nil {
			out.Error = errMsg.Error.Error()
		}
		if out.Error == "" {
			out.Error = http.StatusText(errMsg.StatusCode)
		}
		c.JSON(http.StatusOK, out)
		return
	}

	out.OK = true
	out.StatusCode = http.StatusOK
	out.RawResponse = string(resp)
	out.Text = extractOpenAIResponsesText(resp)
	c.JSON(http.StatusOK, out)
}

func (h *Handler) authByName(name string) *auth.Auth {
	name = strings.TrimSpace(name)
	if name == "" || h == nil || h.authManager == nil {
		return nil
	}
	auths := h.authManager.List()
	for _, item := range auths {
		if item == nil {
			continue
		}
		itemName := authFileName(item)
		if strings.EqualFold(itemName, name) || strings.EqualFold(strings.TrimSpace(item.ID), name) {
			return item
		}
	}
	return nil
}

func authFileName(item *auth.Auth) string {
	if item == nil {
		return ""
	}
	if name := strings.TrimSpace(item.FileName); name != "" {
		return name
	}
	return strings.TrimSpace(item.ID)
}

func normalizeAuthFileTestModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return defaultAuthFileTestModel
	}
	return model
}

func extractOpenAIResponsesText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	if text := strings.TrimSpace(gjson.GetBytes(raw, "output_text").String()); text != "" {
		return text
	}
	var parts []string
	gjson.GetBytes(raw, "output").ForEach(func(_, item gjson.Result) bool {
		content := item.Get("content")
		if !content.IsArray() {
			return true
		}
		content.ForEach(func(_, part gjson.Result) bool {
			if text := strings.TrimSpace(part.Get("text").String()); text != "" {
				parts = append(parts, text)
			}
			return true
		})
		return true
	})
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
