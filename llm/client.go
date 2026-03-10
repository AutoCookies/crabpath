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
			Timeout: 120 * time.Second, // LLM calls can be slow
		},
	}
}

// ─── Request / Response types live here too for decoupling ───────────────────

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

// Complete sends a non-streaming chat completion to cheese-server and returns
// the raw text of the first choice.
func (c *Client) Complete(ctx context.Context, req Request) (string, error) {
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
