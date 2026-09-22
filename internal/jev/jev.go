// Package jev calls TypeSafe AI's Jev model — a "System One" judgment-only
// model that returns typed, calibrated decisions (choice / score / noul)
// instead of generated text. Revita uses it as the fast, low-cost Phase 1
// screening step ahead of a large LLM's detailed Phase 2 report.
//
// API reference (as published by TypeSafe AI): POST
// https://api.typesafe.ai/v1/systemone with an "Authorization: Bearer
// <key>" header. A request carries a "state" (the context to judge) and
// a map of "questions", each of type "choice" (pick from a fixed set of
// criteria), "score" (a continuous rating), or "noul" (a yes/no
// probability — short for Bernoulli). The response echoes back a
// same-shaped "answers" map.
package jev

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL        = "https://api.typesafe.ai/v1/systemone"
	defaultModel   = "jev-latest"
	requestTimeout = 15 * time.Second
)

// Question types.
const (
	TypeChoice = "choice"
	TypeScore  = "score"
	TypeNoul   = "noul"
)

// Question is one typed decision Jev is asked to make about the State.
type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria,omitempty"` // choice型のみ使用
}

// Answer is Jev's typed response to one Question.
type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice,omitempty"`
	Noul          *float64           `json:"noul,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type request struct {
	Model     string              `json:"model"`
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}

type apiResponse struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Client is the low-level Jev API client.
type Client struct {
	APIKey     string
	Model      string // 未設定時は defaultModel を使用
	HTTPClient *http.Client

	testBaseURL string // テスト時にbaseURLを差し替える
}

// NewClient returns a Client authenticated with the given TypeSafe AI
// API key.
func NewClient(apiKey string) *Client {
	return &Client{APIKey: apiKey, Model: defaultModel}
}

// Evaluate sends state and questions to Jev and returns its typed
// answers, keyed by question ID.
func (c *Client) Evaluate(state string, questions map[string]Question) (map[string]Answer, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("jev: api key is not configured")
	}

	model := c.Model
	if model == "" {
		model = defaultModel
	}

	body, err := json.Marshal(request{Model: model, State: state, Questions: questions})
	if err != nil {
		return nil, err
	}

	endpoint := baseURL
	if c.testBaseURL != "" {
		endpoint = c.testBaseURL
	}

	httpReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("jev: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var parsed apiResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return nil, fmt.Errorf("jev: failed to parse response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("jev: api error: %s", parsed.Error.Message)
	}
	if len(parsed.Answers) == 0 {
		return nil, fmt.Errorf("jev: api returned no answers")
	}
	return parsed.Answers, nil
}
