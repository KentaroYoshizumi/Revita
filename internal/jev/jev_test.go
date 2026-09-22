package jev

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEvaluate_ChoiceNoulScore(t *testing.T) {
	var capturedBody request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-key")
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		fmt.Fprint(w, `{
			"model": "jev-1.13.0",
			"answers": {
				"verdict": {"type": "choice", "choice": "Go", "confidence": 0.91, "probabilities": {"Go": 0.91, "Conditional": 0.07, "NoGo": 0.02}},
				"roi_sufficient": {"type": "noul", "noul": 0.88},
				"score_axis": {"type": "score", "score": 1.4, "confidence": 0.7}
			},
			"usage": {"input_tokens": 300, "output_tokens": 0}
		}`)
	}))
	defer server.Close()

	c := &Client{APIKey: "test-key", Model: "jev-latest", HTTPClient: server.Client(), testBaseURL: server.URL}

	answers, err := c.Evaluate("some state text", map[string]Question{
		"verdict": {Type: TypeChoice, Instructions: "judge", Criteria: map[string]string{"Go": "", "Conditional": "", "NoGo": ""}},
	})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}

	if capturedBody.Model != "jev-latest" {
		t.Errorf("request model = %q, want %q", capturedBody.Model, "jev-latest")
	}

	verdict, ok := answers["verdict"]
	if !ok {
		t.Fatal("missing verdict answer")
	}
	if verdict.Choice != "Go" {
		t.Errorf("verdict.Choice = %q, want %q", verdict.Choice, "Go")
	}
	if verdict.Confidence == nil || *verdict.Confidence != 0.91 {
		t.Errorf("verdict.Confidence = %v, want 0.91", verdict.Confidence)
	}

	roi, ok := answers["roi_sufficient"]
	if !ok || roi.Noul == nil || *roi.Noul != 0.88 {
		t.Errorf("roi_sufficient answer = %+v, want noul=0.88", roi)
	}

	score, ok := answers["score_axis"]
	if !ok || score.Score == nil || *score.Score != 1.4 {
		t.Errorf("score_axis answer = %+v, want score=1.4", score)
	}
}

func TestEvaluate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"error": {"message": "invalid api key"}}`)
	}))
	defer server.Close()

	c := &Client{APIKey: "bad-key", HTTPClient: server.Client(), testBaseURL: server.URL}
	if _, err := c.Evaluate("state", map[string]Question{}); err == nil {
		t.Fatal("expected an error when the API reports an error")
	}
}

func TestEvaluate_MissingAPIKey(t *testing.T) {
	c := &Client{}
	if _, err := c.Evaluate("state", map[string]Question{}); err == nil {
		t.Fatal("expected an error when APIKey is not configured")
	}
}
