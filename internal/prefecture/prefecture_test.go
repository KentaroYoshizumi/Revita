package prefecture

import "testing"

func TestFromAddress(t *testing.T) {
	cases := []struct {
		address  string
		wantName string
		wantCode string
		wantOK   bool
	}{
		{"東京都渋谷区神南1-1-1", "東京都", "13", true},
		{"福岡県福岡市中央区天神1-1", "福岡県", "40", true},
		{"徳島県徳島市寺島本町西1-1", "徳島県", "36", true},
		{"大阪府大阪市北区梅田1-1-1", "大阪府", "27", true},
		{"存在しない住所", "", "", false},
	}

	for _, c := range cases {
		got, ok := FromAddress(c.address)
		if ok != c.wantOK {
			t.Errorf("FromAddress(%q) ok = %v, want %v", c.address, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if got.Name != c.wantName || got.Code != c.wantCode {
			t.Errorf("FromAddress(%q) = %+v, want {%s %s}", c.address, got, c.wantName, c.wantCode)
		}
	}
}
