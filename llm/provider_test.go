package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmcarbo/datjitgo/core/ports"
)

func TestHTTPProviderOpenAICompatible(t *testing.T) {
	var gotPath, gotAuth, gotModel, gotPrompt string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotModel = body.Model
		gotPrompt = body.Messages[0].Content
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"live text"}}]}`))
	}))
	defer srv.Close()

	got, err := NewHTTP().Complete(context.Background(), ports.LLMRequest{
		Provider: "openai",
		Endpoint: srv.URL + "/v1",
		Model:    "gpt-test",
		APIKey:   "secret",
		Prompt:   "write",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "live text" || gotPath != "/v1/chat/completions" || gotAuth != "Bearer secret" || gotModel != "gpt-test" || gotPrompt != "write" {
		t.Fatalf("unexpected request/response got=%q path=%q auth=%q model=%q prompt=%q", got, gotPath, gotAuth, gotModel, gotPrompt)
	}
}

func TestHTTPProviderOllama(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"response":"ollama text"}`))
	}))
	defer srv.Close()

	got, err := NewHTTP().Complete(context.Background(), ports.LLMRequest{
		Provider: "ollama",
		Endpoint: srv.URL,
		Model:    "llama-test",
		Prompt:   "write",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ollama text" || gotPath != "/api/generate" {
		t.Fatalf("unexpected got=%q path=%q", got, gotPath)
	}
}
