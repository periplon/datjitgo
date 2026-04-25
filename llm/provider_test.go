package llm

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	coreerrors "github.com/jmcarbo/datjitgo/core/errors"
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

func TestHTTPProviderOpenAICompatibleTextFallbackAndOptions(t *testing.T) {
	var gotTemp, gotMax any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer env-secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		gotTemp = body["temperature"]
		gotMax = body["max_tokens"]
		_, _ = w.Write([]byte(`{"choices":[{"text":" fallback text "}]}`))
	}))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "env-secret")
	temp := 0.25
	maxTokens := 12

	got, err := NewHTTP().Complete(context.Background(), ports.LLMRequest{
		Endpoint:    srv.URL,
		Model:       "gpt-test",
		Prompt:      "write",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TimeoutSecs: intPtr(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "fallback text" || gotTemp != temp || gotMax != float64(maxTokens) {
		t.Fatalf("got=%q temp=%#v max=%#v", got, gotTemp, gotMax)
	}
}

func TestHTTPProviderReturnsGenerationErrors(t *testing.T) {
	p := NewHTTP()
	cases := []struct {
		name string
		req  ports.LLMRequest
	}{
		{name: "unsupported", req: ports.LLMRequest{Provider: "wat", Model: "m"}},
		{name: "missing-openai-model", req: ports.LLMRequest{Provider: "openai"}},
		{name: "missing-ollama-model", req: ports.LLMRequest{Provider: "ollama"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.Complete(context.Background(), tc.req)
			if !stderrors.Is(err, coreerrors.ErrGeneration) {
				t.Fatalf("error = %v, want generation", err)
			}
		})
	}
}

func TestHTTPProviderResponseErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		code int
		req  ports.LLMRequest
	}{
		{name: "openai-no-choices", body: `{"choices":[]}`, req: ports.LLMRequest{Provider: "openai", Model: "m"}},
		{name: "openai-empty-content", body: `{"choices":[{"message":{"content":" "},"text":" "}]}`, req: ports.LLMRequest{Provider: "openai", Model: "m"}},
		{name: "ollama-error", body: `{"error":"bad model"}`, req: ports.LLMRequest{Provider: "ollama", Model: "m"}},
		{name: "ollama-empty", body: `{"response":" "}`, req: ports.LLMRequest{Provider: "ollama", Model: "m"}},
		{name: "http-status", body: `nope`, code: http.StatusBadGateway, req: ports.LLMRequest{Provider: "openai", Model: "m"}},
		{name: "bad-json", body: `{`, req: ports.LLMRequest{Provider: "openai", Model: "m"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if tt.code != 0 {
					w.WriteHeader(tt.code)
				}
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			tt.req.Endpoint = srv.URL
			_, err := NewHTTP().Complete(context.Background(), tt.req)
			if !stderrors.Is(err, coreerrors.ErrGeneration) {
				t.Fatalf("error = %v, want generation", err)
			}
		})
	}
}

func TestHTTPProviderDefaultsAndBearerKeys(t *testing.T) {
	if got := defaultOpenAIEndpoint("lmstudio"); got != "http://localhost:1234/v1" {
		t.Fatalf("lmstudio endpoint = %q", got)
	}
	if got := defaultOpenAIEndpoint("lm-studio"); got != "http://localhost:1234/v1" {
		t.Fatalf("lm-studio endpoint = %q", got)
	}
	if got := defaultOpenAIEndpoint("vllm"); got != "http://localhost:8000/v1" {
		t.Fatalf("vllm endpoint = %q", got)
	}
	if got := defaultOpenAIEndpoint("openai"); got != "https://api.openai.com/v1" {
		t.Fatalf("openai endpoint = %q", got)
	}

	t.Setenv("DATJIT_LLM_API_KEY", "datjit-secret")
	if got := bearerKey(ports.LLMRequest{Provider: "vllm"}); got != "datjit-secret" {
		t.Fatalf("provider bearer = %q", got)
	}
	if got := bearerKey(ports.LLMRequest{Provider: "openai", APIKey: "request-secret"}); got != "request-secret" {
		t.Fatalf("request bearer = %q", got)
	}
	if err := llmErr("wrapped", stderrors.New("cause")); !stderrors.Is(err, coreerrors.ErrGeneration) || !strings.Contains(err.Error(), "wrapped") {
		t.Fatalf("llmErr = %v, want generation wrapper", err)
	}
}

func intPtr(v int) *int {
	return &v
}
