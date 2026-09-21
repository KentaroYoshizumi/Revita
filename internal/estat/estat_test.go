package estat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOccupancyRate_ArrayValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"GET_STATS_DATA": {
				"RESULT": {"STATUS": 0, "ERROR_MSG": "OK"},
				"STATISTICAL_DATA": {
					"DATA_INF": {
						"VALUE": [
							{"@area": "13000", "@time": "2023000000", "$": "70.1"},
							{"@area": "13000", "@time": "2024000000", "$": "75.3"},
							{"@area": "27000", "@time": "2024000000", "$": "60.0"}
						]
					}
				}
			}
		}`)
	}))
	defer server.Close()

	c := &Client{AppID: "id", StatsDataID: "stats", HTTPClient: server.Client()}
	c.testBaseURL = server.URL

	rate, err := c.GetOccupancyRate("13000")
	if err != nil {
		t.Fatalf("GetOccupancyRate returned error: %v", err)
	}
	if rate != 0.753 {
		t.Errorf("GetOccupancyRate = %v, want 0.753 (should pick the latest @time entry)", rate)
	}
}

func TestGetOccupancyRate_SingleValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{
			"GET_STATS_DATA": {
				"RESULT": {"STATUS": 0, "ERROR_MSG": "OK"},
				"STATISTICAL_DATA": {
					"DATA_INF": {
						"VALUE": {"@area": "40000", "@time": "2024000000", "$": "65.4"}
					}
				}
			}
		}`)
	}))
	defer server.Close()

	c := &Client{AppID: "id", StatsDataID: "stats", HTTPClient: server.Client()}
	c.testBaseURL = server.URL

	rate, err := c.GetOccupancyRate("40000")
	if err != nil {
		t.Fatalf("GetOccupancyRate returned error: %v", err)
	}
	if rate != 0.654 {
		t.Errorf("GetOccupancyRate = %v, want 0.654", rate)
	}
}

func TestGetOccupancyRate_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"GET_STATS_DATA": {"RESULT": {"STATUS": 1, "ERROR_MSG": "パラメータが不正です。"}}}`)
	}))
	defer server.Close()

	c := &Client{AppID: "id", StatsDataID: "stats", HTTPClient: server.Client()}
	c.testBaseURL = server.URL

	if _, err := c.GetOccupancyRate("13000"); err == nil {
		t.Fatal("expected an error when the API reports a non-zero STATUS")
	}
}

func TestGetOccupancyRate_MissingCredentials(t *testing.T) {
	c := &Client{}
	if _, err := c.GetOccupancyRate("13000"); err == nil {
		t.Fatal("expected an error when AppID/StatsDataID are not configured")
	}
}
