package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAIEmbeddingReturnsVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/embeddings" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("authorization leaked or missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer server.Close()

	model, err := NewOpenAIEmbedding(EmbeddingConfig{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "text-embedding-3-small",
		Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	vectors, err := model.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vectors) != 1 || len(vectors[0]) != 3 {
		t.Fatalf("vectors = %#v", vectors)
	}
}

func TestOpenAIEmbeddingMaps5xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()
	model, err := NewOpenAIEmbedding(EmbeddingConfig{BaseURL: server.URL, APIKey: "k", Model: "m", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = model.Embed(context.Background(), []string{"x"})
	if err == nil {
		t.Fatal("expected error")
	}
}
