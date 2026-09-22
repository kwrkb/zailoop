package site

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/render"
	"github.com/kwrkb/zailoop/internal/rs"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed assets/*
var assetFS embed.FS

var funcs = template.FuncMap{
	"yen":   render.Yen,
	"ratio": render.Ratio,
	"term":  render.TermLabel,
	"metric": func(o lifecycle.OutcomeLine) string {
		return render.MetricLabel(o)
	},
	"state": func(s lifecycle.State) string { return s.String() },
	"isOK":  func(s lifecycle.State) bool { return s == lifecycle.StateOK },
	"or": func(s, alt string) string {
		if s == "" {
			return alt
		}
		return s
	},
	"join": strings.Join,
	"pct":  func(w float64) string { return fmt.Sprintf("%.1f", w) },
	"ratePct": func(s string) float64 { // 達成率文字列 → 0〜100 に丸めた幅
		v, ok := lifecycle.ParseRate(s)
		if !ok || v < 0 {
			return 0
		}
		if v > 100 {
			return 100
		}
		return v
	},
	"yenSigned": func(v int64) string { return render.YenValue(v) },
	"inc":       func(n int) int { return n + 1 },
	"verdict":   func(v lifecycle.LoopVerdict) string { return v.String() },
	"verdictClass": func(v lifecycle.LoopVerdict) string {
		switch v {
		case lifecycle.VerdictContradiction:
			return "bad"
		case lifecycle.VerdictConsistent:
			return "good"
		}
		return ""
	},
	"execText": func(y lifecycle.YearRow) string {
		switch y.ExecState {
		case lifecycle.StateOK:
			return render.Yen(y.Executed)
		case lifecycle.StateNotComputable:
			return render.Yen(y.Executed) + "（算出対象外）"
		}
		return "未確定"
	},
	"unusedText": func(y lifecycle.YearRow) string {
		switch y.UnusedState {
		case lifecycle.StateOK:
			return render.Yen(y.Unused)
		case lifecycle.StateNeedsReview:
			return "要確認"
		}
		return "—"
	},
}

var templates = template.Must(template.New("").Funcs(funcs).ParseFS(templateFS, "templates/*.html"))

// detailData は detail.html に渡すデータ。
type detailData struct {
	LC          *lifecycle.Lifecycle
	TL          *lifecycle.Timeline
	Multi       bool // 2 年度以上のシートがある
	Summary     lifecycle.Summary
	StageBars   []Bar
	YearBars    []yearBar
	Signals     []lifecycle.SignalDesc
	Generated   string
	Attribution string
	Root        string // index.html への相対パス（"../"）
}

type yearBar struct {
	Year     int
	Initial  Bar
	Current  Bar
	Executed Bar
}

// writeDetail は 1 事業の詳細ページを書く。
func writeDetail(w io.Writer, tl *lifecycle.Timeline, sm lifecycle.Summary, generated string) error {
	lc := tl.Latest
	n := lc.ActualYear
	stage := []BarInput{
		{Label: fmt.Sprintf("① FY%d 概算要求", n), Value: sm.Request, Note: lc.Request.State.String()},
		{Label: fmt.Sprintf("② FY%d 当初予算", n), Value: sm.Initial, Note: lc.Enacted.State.String()},
		{Label: fmt.Sprintf("② FY%d 歳出予算現額", n), Value: sm.Current, Note: lc.Enacted.State.String()},
		{Label: fmt.Sprintf("③ FY%d 執行額", n), Value: lc.Execution.Executed, Note: lc.Execution.State.String()},
		{Label: fmt.Sprintf("④ FY%d 不用相当額", n), Value: sm.Unused, Note: unusedNote(lc.Settlement)},
		{Label: fmt.Sprintf("⑥ FY%d 概算要求", n+2), Value: sm.NextRequest, Note: lc.Reflection.State.String()},
	}
	var ybars []yearBar
	if len(lc.Years) > 0 {
		var in []BarInput
		for _, y := range lc.Years {
			in = append(in, BarInput{Value: y.Initial}, BarInput{Value: y.Current}, BarInput{Value: y.Executed})
		}
		bars := Bars(in, render.Yen)
		for i, y := range lc.Years {
			ybars = append(ybars, yearBar{Year: y.Year, Initial: bars[i*3], Current: bars[i*3+1], Executed: bars[i*3+2]})
		}
	}
	var sigs []lifecycle.SignalDesc
	for _, d := range lifecycle.SignalInfo {
		if sm.Signals.Has(d.Signal) {
			sigs = append(sigs, d)
		}
	}
	return templates.ExecuteTemplate(w, "detail.html", detailData{
		LC: lc, TL: tl, Multi: len(tl.SheetYears) > 1, Summary: sm, StageBars: Bars(stage, render.Yen), YearBars: ybars, Signals: sigs,
		Generated: generated, Attribution: render.Attribution, Root: "../",
	})
}

func unusedNote(st lifecycle.Settlement) string {
	switch st.UnusedState {
	case lifecycle.StateNeedsReview:
		return "算出不能／差額 " + render.YenValue(st.Diff) + "（要確認）"
	case lifecycle.StateNotComputable:
		return "算出対象外"
	}
	return st.UnusedState.String()
}

// yenPtr は JSON 用に空欄を nil にする。
func yenPtr(v rs.Yen) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Value
	return &x
}
