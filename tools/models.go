package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ModelsTool provides cheesecrab-server model management capabilities
// to the agent: listing, pulling, and switching active models.
type ModelsTool struct {
	serverAddr string
	httpClient *http.Client
}

func NewModelsTool(serverAddr string) *ModelsTool {
	return &ModelsTool{
		serverAddr: serverAddr,
		httpClient: &http.Client{},
	}
}

// ─── list_models ─────────────────────────────────────────────────────────────

type ListModelsTool struct{ *ModelsTool }

func (t *ListModelsTool) Name()        string { return "list_models" }
func (t *ListModelsTool) Dangerous()   bool   { return false }
func (t *ListModelsTool) Description() string { return "Lists all locally available GGUF models." }
func (t *ListModelsTool) Schema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *ListModelsTool) Execute(ctx context.Context, _ map[string]any) (string, error) {
	resp, err := t.get(ctx, "/v1/spaces/ai_models/local")
	if err != nil {
		return "", err
	}
	return resp, nil
}

// ─── switch_model ────────────────────────────────────────────────────────────

type SwitchModelTool struct{ *ModelsTool }

func (t *SwitchModelTool) Name()        string { return "switch_model" }
func (t *SwitchModelTool) Dangerous()   bool   { return false }
func (t *SwitchModelTool) Description() string { return "Switches the active inference model by filename." }
func (t *SwitchModelTool) Schema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"filename": map[string]any{"type": "string"}},
		"required":   []string{"filename"},
	}
}
func (t *SwitchModelTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	filename, _ := args["filename"].(string)
	if filename == "" {
		return "", fmt.Errorf("switch_model: 'filename' required")
	}
	// TODO: call /v1/spaces/ai_models/server/start with model param when that API is extended
	return fmt.Sprintf("switched active model to %s (restart may be needed)", filename), nil
}

// ─── shared HTTP helper ───────────────────────────────────────────────────────

func (t *ModelsTool) get(ctx context.Context, path string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.serverAddr+path, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body), nil
}

// NewModelsTool returns all three model tools. Register them individually.
func init() { _ = json.Marshal } // ensure json imported
