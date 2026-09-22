// Package llm generates the Phase 2 detailed advisory report for a
// property, using a large LLM (Claude or OpenAI). It is the second step
// of Revita's 2-phase pipeline: Phase 1 (see internal/jev) produces a
// fast, low-cost Go/Conditional/NoGo screening verdict from the
// Go-calculated numbers; this package turns that verdict, together with
// the same numbers, into a detailed Markdown report for the user.
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/KentaroYoshizumi/Revita/internal/airdna"
	"github.com/KentaroYoshizumi/Revita/internal/finance"
	"github.com/KentaroYoshizumi/Revita/internal/jev"
	"github.com/KentaroYoshizumi/Revita/internal/property"
)

const requestTimeout = 30 * time.Second

// GenerateReport asks an LLM to produce a detailed Japanese Markdown
// advisory report for the property, given Jev's Phase-1 verdict as
// context.
//
// It prefers the Claude API (ANTHROPIC_API_KEY), falls back to the
// OpenAI API (OPENAI_API_KEY) if that is not set, and falls back to
// SimpleSummary if neither API key is configured, so the CLI remains
// usable without any network access.
func GenerateReport(p property.Property, m airdna.MarketData, r finance.Result, eval jev.Evaluation) (string, error) {
	prompt := buildReportPrompt(p, m, r, eval)

	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return callClaude(key, prompt)
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return callOpenAI(key, prompt)
	}
	return SimpleSummary(eval, r), nil
}

func buildReportPrompt(p property.Property, m airdna.MarketData, r finance.Result, eval jev.Evaluation) string {
	return fmt.Sprintf(`あなたは不動産投資（特にAirbnbなどの短期賃貸運用）の専門アナリストです。
以下の物件情報・市場データ・収支計算結果・一次判定（Phase 1）の結果を踏まえて、
ユーザー向けの詳細な投資判断レポートをMarkdown形式で作成してください。

レポートには以下の見出しを含めてください:
## 総合判定
## 数値の解説（ROI・表面/実質利回り・損益分岐稼働率）
## リスクと留意点
## 次に取るべきアクション

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
- 表面利回り: %.2f%%
- 実質利回り: %.2f%%
- 損益分岐稼働率: %.1f%%

【Phase 1（Jev）の一次判定】
- 判定: %s（確度 %.0f%%）
- 判定軸ごとの確度: %s
`,
		p.Address, p.PurchasePrice, p.MonthlyRent, p.SizeSqm, p.Capacity,
		m.ADR, m.OccupancyRate*100, m.CompetitorCount, m.DataSource,
		r.MonthlyRevenue, r.MonthlyProfit, r.AnnualProfit, r.ROIPercent,
		r.GrossYieldPercent, r.NetYieldPercent, r.BEPOccupancyRate*100,
		eval.Verdict, eval.Confidence*100, formatReasons(eval.Reasons),
	)
}

// formatReasons renders Jev's per-axis confidence scores as a stable,
// human-readable list.
func formatReasons(reasons map[string]float64) string {
	if len(reasons) == 0 {
		return "(なし)"
	}
	keys := make([]string, 0, len(reasons))
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := ""
	for i, k := range keys {
		if i > 0 {
			out += ", "
		}
		out += fmt.Sprintf("%s=%.2f", k, reasons[k])
	}
	return out
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
		MaxTokens: 1536,
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

// SimpleSummary renders a short Markdown summary directly from Jev's
// Phase-1 evaluation, without calling any LLM. The pipeline uses this
// both as the NoGo cost-saving shortcut (skipping Phase 2 entirely) and
// as the offline fallback when no LLM API key is configured.
func SimpleSummary(eval jev.Evaluation, r finance.Result) string {
	return fmt.Sprintf(
		"## 総合判定\n\n**%s**（確度 %.0f%%）\n\n"+
			"## 数値の解説\n\n"+
			"- 想定月間損益: %.0f円\n"+
			"- 年間ROI: %.2f%%\n"+
			"- 表面利回り: %.2f%%\n"+
			"- 実質利回り: %.2f%%\n"+
			"- 損益分岐稼働率: %.1f%%\n\n"+
			"## 補足\n\n"+
			"これはPhase 1（Jev）の判定のみに基づく簡易サマリーです。%s\n",
		eval.Verdict, eval.Confidence*100,
		r.MonthlyProfit, r.ROIPercent, r.GrossYieldPercent, r.NetYieldPercent, r.BEPOccupancyRate*100,
		simpleSummaryNote(eval),
	)
}

func simpleSummaryNote(eval jev.Evaluation) string {
	if eval.Verdict == jev.VerdictNoGo {
		return "NoGo判定のため、Phase 2（LLMによる詳細レポート生成）はコスト削減のためスキップされています。"
	}
	return "LLM APIキー（ANTHROPIC_API_KEY または OPENAI_API_KEY）が未設定のため、詳細レポート生成をスキップしています。"
}
