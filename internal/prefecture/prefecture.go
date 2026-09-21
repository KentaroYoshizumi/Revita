// Package prefecture maps Japanese prefecture names to their JIS X 0401
// prefecture codes, which government statistics APIs such as e-Stat use
// to identify areas.
package prefecture

import "strings"

// Prefecture identifies one of Japan's 47 prefectures.
type Prefecture struct {
	Name string // 都道府県名（例: 東京都）
	Code string // JIS X 0401 都道府県コード（2桁、例: "13"）
}

var all = []Prefecture{
	{"北海道", "01"}, {"青森県", "02"}, {"岩手県", "03"}, {"宮城県", "04"},
	{"秋田県", "05"}, {"山形県", "06"}, {"福島県", "07"}, {"茨城県", "08"},
	{"栃木県", "09"}, {"群馬県", "10"}, {"埼玉県", "11"}, {"千葉県", "12"},
	{"東京都", "13"}, {"神奈川県", "14"}, {"新潟県", "15"}, {"富山県", "16"},
	{"石川県", "17"}, {"福井県", "18"}, {"山梨県", "19"}, {"長野県", "20"},
	{"岐阜県", "21"}, {"静岡県", "22"}, {"愛知県", "23"}, {"三重県", "24"},
	{"滋賀県", "25"}, {"京都府", "26"}, {"大阪府", "27"}, {"兵庫県", "28"},
	{"奈良県", "29"}, {"和歌山県", "30"}, {"鳥取県", "31"}, {"島根県", "32"},
	{"岡山県", "33"}, {"広島県", "34"}, {"山口県", "35"}, {"徳島県", "36"},
	{"香川県", "37"}, {"愛媛県", "38"}, {"高知県", "39"}, {"福岡県", "40"},
	{"佐賀県", "41"}, {"長崎県", "42"}, {"熊本県", "43"}, {"大分県", "44"},
	{"宮崎県", "45"}, {"鹿児島県", "46"}, {"沖縄県", "47"},
}

// FromAddress returns the prefecture whose name appears in address, and
// true if one was found.
func FromAddress(address string) (Prefecture, bool) {
	for _, p := range all {
		if strings.Contains(address, p.Name) {
			return p, true
		}
	}
	return Prefecture{}, false
}
