// Package llm contains HTTP-backed providers for @llm generation.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jmcarbo/datjitgo/core/errors"
	"github.com/jmcarbo/datjitgo/core/ports"
)

// HTTPProvider calls OpenAI-compatible endpoints (openai, lmstudio, vllm) and
// Ollama's native /api/generate endpoint.
type HTTPProvider struct {
	Client *http.Client
}

// NewHTTP returns a provider using a client with a finite default timeout.
func NewHTTP() *HTTPProvider {
	return &HTTPProvider{Client: &http.Client{Timeout: 60 * time.Second}}
}

// Complete implements ports.LLMProvider.
func (p *HTTPProvider) Complete(ctx context.Context, req ports.LLMRequest) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.TimeoutSecs != nil && *req.TimeoutSecs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*req.TimeoutSecs)*time.Second)
		defer cancel()
	}

	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "openai"
	}
	switch provider {
	case "openai", "lmstudio", "lm-studio", "vllm":
		return p.completeOpenAICompatible(ctx, req, provider)
	case "ollama":
		return p.completeOllama(ctx, req)
	default:
		return "", llmErr(fmt.Sprintf("unsupported llm provider %q", req.Provider), nil)
	}
}

func (p *HTTPProvider) completeOpenAICompatible(ctx context.Context, req ports.LLMRequest, provider string) (string, error) {
	endpoint := strings.TrimRight(req.Endpoint, "/")
	if endpoint == "" {
		endpoint = defaultOpenAIEndpoint(provider)
	}
	model := req.Model
	if strings.TrimSpace(model) == "" {
		return "", llmErr("llm model is required", nil)
	}
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": req.Prompt},
		},
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			Text string `json:"text"`
		} `json:"choices"`
		Error any `json:"error"`
	}
	if err := p.postJSON(ctx, endpoint+"/chat/completions", bearerKey(req), body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", llmErr("llm response contained no choices", nil)
	}
	text := strings.TrimSpace(out.Choices[0].Message.Content)
	if text == "" {
		text = strings.TrimSpace(out.Choices[0].Text)
	}
	if text == "" {
		return "", llmErr("llm response contained empty content", nil)
	}
	return text, nil
}

func (p *HTTPProvider) completeOllama(ctx context.Context, req ports.LLMRequest) (string, error) {
	endpoint := strings.TrimRight(req.Endpoint, "/")
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if strings.TrimSpace(req.Model) == "" {
		return "", llmErr("ollama model is required", nil)
	}
	body := map[string]any{
		"model":  req.Model,
		"prompt": req.Prompt,
		"stream": false,
	}
	if req.Temperature != nil {
		body["options"] = map[string]any{"temperature": *req.Temperature}
	}
	var out struct {
		Response string `json:"response"`
		Error    string `json:"error"`
	}
	if err := p.postJSON(ctx, endpoint+"/api/generate", "", body, &out); err != nil {
		return "", err
	}
	if out.Error != "" {
		return "", llmErr(out.Error, nil)
	}
	text := strings.TrimSpace(out.Response)
	if text == "" {
		return "", llmErr("ollama response contained empty content", nil)
	}
	return text, nil
}

func (p *HTTPProvider) postJSON(ctx context.Context, url, bearer string, body any, out any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return llmErr("encode llm request", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return llmErr("build llm request", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return llmErr("send llm request", err)
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return llmErr("read llm response", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return llmErr(fmt.Sprintf("llm request failed: %s: %s", resp.Status, strings.TrimSpace(string(payload))), nil)
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return llmErr("decode llm response", err)
	}
	return nil
}

func defaultOpenAIEndpoint(provider string) string {
	switch provider {
	case "lmstudio", "lm-studio":
		return "http://localhost:1234/v1"
	case "vllm":
		return "http://localhost:8000/v1"
	default:
		return "https://api.openai.com/v1"
	}
}

func bearerKey(req ports.LLMRequest) string {
	if req.APIKey != "" {
		return req.APIKey
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "openai" || provider == "" {
		return os.Getenv("OPENAI_API_KEY")
	}
	return os.Getenv("DATJIT_LLM_API_KEY")
}

func llmErr(msg string, cause error) error {
	return &errors.Error{Kind: errors.KindGeneration, Message: msg, Cause: cause}
}

var _ ports.LLMProvider = (*HTTPProvider)(nil)
