// Package minpaku loads prefecture-level 住宅宿泊事業（民泊）届出住宅数
// (registered short-term rental listing counts), as published by the
// Japan Tourism Agency's minpaku portal:
// https://www.mlit.go.jp/kankocho/minpaku/business/host/construction_situation.html
//
// The agency publishes this as a periodic report rather than a live
// API, so this package reads it from a local CSV snapshot that the user
// downloads and updates by hand.
package minpaku

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
)

// Counts maps prefecture name (e.g. "東京都") to its registered minpaku
// listing count.
type Counts map[string]int

// LoadCSV reads a two-column CSV (都道府県,届出住宅数, with a header row)
// exported from the Japan Tourism Agency's published figures.
func LoadCSV(path string) (Counts, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("minpaku: failed to read %s: %w", path, err)
	}

	counts := Counts{}
	for i, row := range rows {
		if i == 0 || len(row) < 2 {
			continue // header row
		}
		n, err := strconv.Atoi(row[1])
		if err != nil {
			return nil, fmt.Errorf("minpaku: invalid count on row %d of %s: %w", i+1, path, err)
		}
		counts[row[0]] = n
	}
	return counts, nil
}
