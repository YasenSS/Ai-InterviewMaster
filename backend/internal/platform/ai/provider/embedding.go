package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	platformai "github.com/interviewmaster/interviewmaster/backend/internal/platform/ai"
)

type EmbeddingConfig struct {
	Provider string
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
}

type OpenAIEmbedding struct {
	client   *http.Client
	baseURL  string
	apiKey   string
	model    string
	provider string
}

func NewOpenAIEmbedding(cfg EmbeddingConfig) (*OpenAIEmbedding, error) {
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, &platformai.Error{Code: platformai.ErrorNotConfigured, Cause: errors.New("embedding model is not configured")}
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	providerName := strings.TrimSpace(cfg.Provider)
	if providerName == "" {
		providerName = "openai"
	}
	return &OpenAIEmbedding{
		client:   &http.Client{Timeout: timeout},
		baseURL:  baseURL,
		apiKey:   cfg.APIKey,
		model:    strings.TrimSpace(cfg.Model),
		provider: providerName,
	}, nil
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

func (e *OpenAIEmbedding) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}
	body, err := json.Marshal(embeddingRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &platformai.Error{Code: platformai.ErrorTimeout, Retryable: true, Cause: err}
		}
		return nil, &platformai.Error{Code: platformai.ErrorProviderUnavailable, Retryable: true, Cause: err}
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, &platformai.Error{Code: platformai.ErrorRateLimited, Retryable: true, Cause: fmt.Errorf("embeddings status %d", resp.StatusCode)}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, &platformai.Error{Code: platformai.ErrorAuthentication, Cause: fmt.Errorf("embeddings status %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout {
		return nil, &platformai.Error{Code: platformai.ErrorProviderUnavailable, Retryable: true, Cause: fmt.Errorf("embeddings status %d", resp.StatusCode)}
	}
	if resp.StatusCode >= 300 {
		return nil, &platformai.Error{Code: platformai.ErrorInvalidRequest, Cause: fmt.Errorf("embeddings status %d", resp.StatusCode)}
	}
	var parsed embeddingResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, &platformai.Error{Code: platformai.ErrorOutputInvalid, Cause: err}
	}
	vectors := make([][]float32, 0, len(parsed.Data))
	for _, item := range parsed.Data {
		vectors = append(vectors, item.Embedding)
	}
	return vectors, nil
}
