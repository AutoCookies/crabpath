// Package llm provides a thin HTTP client to call cheese-server
// (an OpenAI-compatible local LLM server) from the crabpath agent.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client calls cheese-server's OpenAI-compatible endpoint.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new LLM client pointed at the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ─── Request / Response types ─────────────────────────────────────────────────

type Request struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Grammar  string    `json:"grammar,omitempty"`
	Stream   bool      `json:"stream"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// ─── Model discovery ──────────────────────────────────────────────────────────

type modelListResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// GetActiveModel queries cheese-server for loaded models and returns the first
// one. If none are loaded or the server is unreachable it returns "".
func (c *Client) GetActiveModel(ctx context.Context) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v1/models", nil)
	if err != nil {
		return ""
	}
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var list modelListResponse
	if err := json.Unmarshal(raw, &list); err != nil || len(list.Data) == 0 {
		return ""
	}
	return list.Data[0].ID
}

// ─── Chat completion ──────────────────────────────────────────────────────────

// Complete sends a non-streaming chat completion to cheese-server and returns
// the raw text of the first choice. If req.Model is empty or "default", it
// auto-detects the active model via GetActiveModel.
func (c *Client) Complete(ctx context.Context, req Request) (string, error) {
	if req.Model == "" || req.Model == "default" {
		if active := c.GetActiveModel(ctx); active != "" {
			req.Model = active
		} else {
			return "", fmt.Errorf("crabpath/llm: no model loaded in cheese-server — please select a model in AI Models space first")
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("crabpath/llm: marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("crabpath/llm: new request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("crabpath/llm: http do: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("crabpath/llm: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("crabpath/llm: cheese-server %d: %s", resp.StatusCode, string(raw))
	}

	var llmResp Response
	if err := json.Unmarshal(raw, &llmResp); err != nil {
		return "", fmt.Errorf("crabpath/llm: unmarshal: %w", err)
	}
	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("crabpath/llm: empty choices from cheese-server")
	}

	return llmResp.Choices[0].Message.Content, nil
}
