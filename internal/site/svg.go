package site

import "github.com/kwrkb/zailoop/internal/rs"

// BarInput は横バー 1 本分の入力。
type BarInput struct {
	Label string
	Value rs.Yen
	Note  string // 値が無いときに表示する説明（データなし/算出対象外 など）
}

// Bar はテンプレートが描く横バー。Width は 0〜100 の百分率。
type Bar struct {
	Label string
	Text  string  // 表示する金額文字列
	Class string  // "", "neg", "none"
	Width float64 // 0〜100
	Valid bool
}

// Bars は最大絶対値を 100% として各バーの幅を求める純粋関数。
// 負値は Class "neg"、値なしは Class "none" で幅 0。
func Bars(items []BarInput, format func(rs.Yen) string) []Bar {
	var max int64
	for _, it := range items {
		if it.Value.Valid {
			v := it.Value.Value
			if v < 0 {
				v = -v
			}
			if v > max {
				max = v
			}
		}
	}
	out := make([]Bar, 0, len(items))
	for _, it := range items {
		b := Bar{Label: it.Label}
		if !it.Value.Valid {
			b.Class = "none"
			b.Text = it.Note
			if b.Text == "" {
				b.Text = "—"
			}
			out = append(out, b)
			continue
		}
		b.Valid = true
		b.Text = format(it.Value)
		v := it.Value.Value
		if v < 0 {
			b.Class = "neg"
			v = -v
		}
		if max > 0 {
			b.Width = float64(v) / float64(max) * 100
		}
		out = append(out, b)
	}
	return out
}
