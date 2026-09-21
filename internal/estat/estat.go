// Package estat fetches statistics from e-Stat (政府統計の総合窓口),
// Japan's official government statistics portal. It is used to look up
// the hotel/lodging room occupancy rate (客室稼働率) published by the
// Japan Tourism Agency's 宿泊旅行統計調査 survey, broken down by
// prefecture — a free, nationwide substitute for AirDNA's occupancy
// metric.
package estat

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

const (
	baseURL        = "https://api.e-stat.go.jp/rest/3.0/app/json/getStatsData"
	requestTimeout = 15 * time.Second
)

// Client fetches statistics from e-Stat.
//
// AppID is issued for free after registering at
// https://www.e-stat.go.jp/mypage/ . StatsDataID identifies the specific
// statistics table to query; find the current 客室稼働率（都道府県別）table
// via e-Stat's site search and copy its ID from the table's "API" tab —
// the government periodically republishes this table under a new ID as
// each fiscal year's figures are finalized, so this must be kept in sync
// manually.
type Client struct {
	AppID       string
	StatsDataID string
	HTTPClient  *http.Client

	// testBaseURL overrides baseURL in tests; empty means use baseURL.
	testBaseURL string
}

// NewClient returns a Client configured with the given e-Stat
// application ID and statistics table ID.
func NewClient(appID, statsDataID string) *Client {
	return &Client{AppID: appID, StatsDataID: statsDataID}
}

type valueEntry struct {
	Area string `json:"@area"`
	Time string `json:"@time"`
	Text string `json:"$"`
}

type statsDataResponse struct {
	GetStatsData struct {
		Result struct {
			Status   int    `json:"STATUS"`
			ErrorMsg string `json:"ERROR_MSG"`
		} `json:"RESULT"`
		StatisticalData struct {
			DataInf struct {
				Value json.RawMessage `json:"VALUE"`
			} `json:"DATA_INF"`
		} `json:"STATISTICAL_DATA"`
	} `json:"GET_STATS_DATA"`
}

// decodeValues handles e-Stat's JSON quirk where the VALUE field is an
// array when there are multiple results but a single object when there
// is exactly one.
func decodeValues(raw json.RawMessage) ([]valueEntry, error) {
	var many []valueEntry
	if err := json.Unmarshal(raw, &many); err == nil {
		return many, nil
	}
	var single valueEntry
	if err := json.Unmarshal(raw, &single); err != nil {
		return nil, err
	}
	return []valueEntry{single}, nil
}

// GetOccupancyRate returns the most recently published room occupancy
// rate (0.0-1.0) for the given 5-digit e-Stat area code. Prefecture-level
// codes end in "000" (e.g. "13000" for Tokyo).
func (c *Client) GetOccupancyRate(areaCode string) (float64, error) {
	if c.AppID == "" {
		return 0, fmt.Errorf("estat: appId is not configured")
	}
	if c.StatsDataID == "" {
		return 0, fmt.Errorf("estat: statsDataId is not configured")
	}

	q := url.Values{}
	q.Set("appId", c.AppID)
	q.Set("statsDataId", c.StatsDataID)
	q.Set("cdArea", areaCode)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: requestTimeout}
	}

	endpoint := baseURL
	if c.testBaseURL != "" {
		endpoint = c.testBaseURL
	}
	resp, err := httpClient.Get(endpoint + "?" + q.Encode())
	if err != nil {
		return 0, fmt.Errorf("estat: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var parsed statsDataResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return 0, fmt.Errorf("estat: failed to parse response: %w", err)
	}
	if parsed.GetStatsData.Result.Status != 0 {
		return 0, fmt.Errorf("estat: api error: %s", parsed.GetStatsData.Result.ErrorMsg)
	}

	values, err := decodeValues(parsed.GetStatsData.StatisticalData.DataInf.Value)
	if err != nil {
		return 0, fmt.Errorf("estat: failed to parse values: %w", err)
	}

	var matched []valueEntry
	for _, v := range values {
		if v.Area == areaCode {
			matched = append(matched, v)
		}
	}
	if len(matched) == 0 {
		return 0, fmt.Errorf("estat: no data found for area code %s", areaCode)
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].Time > matched[j].Time })

	rate, err := strconv.ParseFloat(matched[0].Text, 64)
	if err != nil {
		return 0, fmt.Errorf("estat: failed to parse occupancy rate value %q: %w", matched[0].Text, err)
	}
	return rate / 100, nil
}
