package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

func TestOpenAIGenerateUsesStructuredOutputAndReturnsUsage(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-test",
			"object":"chat.completion",
			"created":1,
			"model":"test-model",
			"choices":[{"index":0,"message":{"role":"assistant","content":"{\"ok\":true}"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}
		}`))
	}))
	defer server.Close()

	chatModel, err := NewOpenAI(context.Background(), OpenAIConfig{
		Provider:          "openai-compatible",
		BaseURL:           server.URL + "/v1",
		APIKey:            "test-key",
		Model:             "test-model",
		Timeout:           2 * time.Second,
		MaxOutputTokens:   1000,
		StructuredOutputs: true,
	})
	if err != nil {
		t.Fatalf("NewOpenAI() error = %v", err)
	}
	temperature := 0.2
	response, err := chatModel.Generate(context.Background(), platformai.GenerateRequest{
		Messages:   []platformai.Message{{Role: platformai.RoleUser, Content: "return JSON"}},
		SchemaName: "test_output",
		JSONSchema: []byte(`{
			"type":"object",
			"properties":{"ok":{"type":"boolean"}},
			"required":["ok"],
			"additionalProperties":false
		}`),
		MaxTokens:   128,
		Temperature: &temperature,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if response.Message.Content != `{"ok":true}` {
		t.Fatalf("content = %q", response.Message.Content)
	}
	if response.Usage.TotalTokens != 14 || response.Provider != "openai-compatible" || response.Model != "test-model" {
		t.Fatalf("response metadata = %#v", response)
	}
	format, ok := requestBody["response_format"].(map[string]any)
	if !ok || format["type"] != "json_schema" {
		t.Fatalf("response_format = %#v", requestBody["response_format"])
	}
	if requestBody["max_completion_tokens"] != float64(128) {
		t.Fatalf("max_completion_tokens = %#v", requestBody["max_completion_tokens"])
	}
}

func TestOpenAIGenerateMapsRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited","type":"rate_limit_error","code":"rate_limit"}}`))
	}))
	defer server.Close()

	chatModel, err := NewOpenAI(context.Background(), OpenAIConfig{
		BaseURL:         server.URL + "/v1",
		APIKey:          "test-key",
		Model:           "test-model",
		Timeout:         2 * time.Second,
		MaxOutputTokens: 100,
	})
	if err != nil {
		t.Fatalf("NewOpenAI() error = %v", err)
	}
	_, err = chatModel.Generate(context.Background(), platformai.GenerateRequest{
		Messages: []platformai.Message{{Role: platformai.RoleUser, Content: "hello"}},
	})
	if !platformai.IsErrorCode(err, platformai.ErrorRateLimited) {
		t.Fatalf("Generate() error = %v, want AI_RATE_LIMITED", err)
	}
}

func TestOpenAIRejectsInvalidMessageBeforeNetwork(t *testing.T) {
	chatModel, err := NewOpenAI(context.Background(), OpenAIConfig{
		APIKey:          "test-key",
		Model:           "test-model",
		Timeout:         time.Second,
		MaxOutputTokens: 100,
	})
	if err != nil {
		t.Fatalf("NewOpenAI() error = %v", err)
	}
	_, err = chatModel.Generate(context.Background(), platformai.GenerateRequest{
		Messages: []platformai.Message{{Role: "invalid", Content: "hello"}},
	})
	if !platformai.IsErrorCode(err, platformai.ErrorInvalidRequest) {
		t.Fatalf("Generate() error = %v, want AI_INVALID_REQUEST", err)
	}
}
