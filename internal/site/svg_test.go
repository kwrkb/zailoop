package site

import (
	"testing"

	"github.com/kwrkb/zailoop/internal/render"
	"github.com/kwrkb/zailoop/internal/rs"
)

func TestBars(t *testing.T) {
	y := func(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }
	got := Bars([]BarInput{
		{Label: "a", Value: y(200)},
		{Label: "b", Value: y(50)},
		{Label: "c", Value: y(-100)},
		{Label: "d", Note: "データなし"},
	}, render.Yen)
	if len(got) != 4 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Width != 100 || got[0].Text != "200 円" || got[0].Class != "" {
		t.Errorf("a = %+v", got[0])
	}
	if got[1].Width != 25 {
		t.Errorf("b = %+v", got[1])
	}
	if got[2].Width != 50 || got[2].Class != "neg" || got[2].Text != "-100 円" {
		t.Errorf("c = %+v", got[2])
	}
	if got[3].Width != 0 || got[3].Class != "none" || got[3].Text != "データなし" || got[3].Valid {
		t.Errorf("d = %+v", got[3])
	}
	if all := Bars([]BarInput{{Label: "z", Value: y(0)}}, render.Yen); all[0].Width != 0 || !all[0].Valid {
		t.Errorf("zero = %+v", all[0])
	}
}
