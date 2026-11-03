package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Client calls the ML service.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 30 * time.Second}}
}

// Embed returns the embedding of text (384 dims, unit length).
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed: ML service returned %d", resp.StatusCode)
	}
	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("embed: decode: %w", err)
	}
	return out.Embedding, nil
}

// Health pings the ML service so a missing service is obvious at startup.
func (c *Client) Health() error {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("embedding service unreachable at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("embedding service at %s returned %d", c.baseURL, resp.StatusCode)
	}
	return nil
}

// FormatVector renders v in pgvector's text format, e.g. "[0.1,0.2]".
func FormatVector(v []float32) string {
	parts := make([]string, len(v))
	for i, x := range v {
		parts[i] = strconv.FormatFloat(float64(x), 'g', -1, 32)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// ParseVector parses pgvector's text format.
func ParseVector(s string) ([]float32, error) {
	s = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), "["), "]")
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	vec := make([]float32, len(parts))
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parse element %d %q: %w", i, p, err)
		}
		vec[i] = float32(v)
	}
	return vec, nil
}
