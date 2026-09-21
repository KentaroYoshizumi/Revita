// Package llm asks a large language model (Claude or OpenAI) to judge
// whether a property is a good investment, given its computed
// profitability figures and the market data behind them.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

const requestTimeout = 30 * time.Second

// Judge asks an LLM whether the property is a profitable investment and
// returns its written verdict in Japanese.
//
// It prefers the Claude API (ANTHROPIC_API_KEY), falls back to the
// OpenAI API (OPENAI_API_KEY) if that is not set, and falls back to a
// local rule-based verdict if neither API key is configured, so the CLI
// remains usable without any network access.
func Judge(p property.Property, m airdna.MarketData, r finance.Result) (string, error) {
	prompt := buildPrompt(p, m, r)

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return callClaude(key, prompt)
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return callOpenAI(key, prompt)
	}
	return localVerdict(r), nil
}

func buildPrompt(p property.Property, m airdna.MarketData, r finance.Result) string {
	return fmt.Sprintf(`あなたは不動産投資（特にAirbnbなどの短期賃貸運用）の専門アナリストです。
以下の物件情報・市場データ・収支計算結果を踏まえて、この物件が「儲かるか・儲からないか」を判定し、
その理由を日本語で3〜5行程度で簡潔に説明してください。

【物件情報】
- 住所: %s
- 購入価格: %.0f円
- 月額費用（賃料・管理費等）: %.0f円
- 広さ: %.1f平米
- 定員: %d名

【該当エリアの市場データ】
- 平均日次単価(ADR): %.0f円
- 稼働率: %.1f%%
- 競合物件数: %d件
- データ出典: %s

【収支計算結果】
- 想定月間収入: %.0f円
- 想定月間損益: %.0f円
- 想定年間損益: %.0f円
- 年間ROI: %.2f%%
`,
		p.Address, p.PurchasePrice, p.MonthlyRent, p.SizeSqm, p.Capacity,
		m.ADR, m.OccupancyRate*100, m.CompetitorCount, m.DataSource,
		r.MonthlyRevenue, r.MonthlyProfit, r.AnnualProfit, r.ROIPercent,
	)
}

// --- Claude (Anthropic Messages API) ---

type claudeRequest struct {
	Model     string          `json:"model"`
	MaxTokens int             `json:"max_tokens"`
	Messages  []claudeMessage `json:"messages"`
}

type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type claudeResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func callClaude(apiKey, prompt string) (string, error) {
	reqBody := claudeRequest{
		Model:     "claude-sonnet-5",
		MaxTokens: 512,
		Messages:  []claudeMessage{{Role: "user", Content: prompt}},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest(http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("claude api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed claudeResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse claude response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("claude api error: %s", parsed.Error.Message)
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("claude api returned no content")
	}
	return parsed.Content[0].Text, nil
}

// --- OpenAI (Chat Completions API) ---

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func callOpenAI(apiKey, prompt string) (string, error) {
	reqBody := openAIRequest{
		Model:    "gpt-4o-mini",
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("openai api request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("failed to parse openai response: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("openai api error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai api returned no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// localVerdict provides a simple rule-based judgement so the CLI still
// produces a useful answer when no LLM API key is configured.
func localVerdict(r finance.Result) string {
	if r.MonthlyProfit > 0 && r.ROIPercent >= 8 {
		return fmt.Sprintf(
			"[ローカル簡易判定] 儲かる可能性が高いと判定します。想定月間損益は%.0f円のプラス、年間ROIは%.2f%%と一般的な投資基準（目安8%%以上）を上回っています。\n"+
				"※この判定はLLM APIキー（ANTHROPIC_API_KEY または OPENAI_API_KEY）が未設定のため、簡易ルールによる代替判定です。",
			r.MonthlyProfit, r.ROIPercent)
	}
	if r.MonthlyProfit > 0 {
		return fmt.Sprintf(
			"[ローカル簡易判定] 月間損益はプラス（%.0f円）ですが、年間ROIは%.2f%%と控えめです。他の物件・条件との比較を推奨します。\n"+
				"※この判定はLLM APIキー（ANTHROPIC_API_KEY または OPENAI_API_KEY）が未設定のため、簡易ルールによる代替判定です。",
			r.MonthlyProfit, r.ROIPercent)
	}
	return fmt.Sprintf(
		"[ローカル簡易判定] 儲からない可能性が高いと判定します。想定月間損益が%.0f円のマイナスとなっており、費用が市場から見込める収入を上回っています。\n"+
			"※この判定はLLM APIキー（ANTHROPIC_API_KEY または OPENAI_API_KEY）が未設定のため、簡易ルールによる代替判定です。",
		r.MonthlyProfit)
}
