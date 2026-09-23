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
	"yenSigned":  func(v int64) string { return render.YenValue(v) },
	"yenShort":   render.YenShort,
	"termID":     termID,
	"disclaimer": func() string { return render.Disclaimer },
	"contradictionNote": render.ContradictionNote,
	"isAttn":            func(v lifecycle.LoopVerdict) bool { return v == lifecycle.VerdictContradiction },
	"inc":        func(n int) int { return n + 1 },
	"verdict":    func(v lifecycle.LoopVerdict) string { return v.String() },
	"verdictClass": func(v lifecycle.LoopVerdict) string {
		switch v {
		case lifecycle.VerdictContradiction:
			return "attn"
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
	Digest      []string
	Flow        []flowStep
	WfIn, WfOut []wfRow
	WfNote      string
	YearBars    []yearBar
	Badges      []badge
	Related     []relatedGroup
	Parents     []relatedLink // AllZero の事業だけ。金額欄の注記に使う
	PastParents []pastParent  // AllZero の事業だけ。古いシートにだけある親事業
	Generated   string
	Root        string // index.html への相対パス（"../"）
}

type yearBar struct {
	Year     int
	Initial  Bar
	Current  Bar
	Executed Bar
}

// relatedLink は関連事業 1 件。Href はページがある事業だけ埋める。
type relatedLink struct {
	ID, Name, Href string
}

// pastParent は古いシートにだけ記載のある親事業。
type pastParent struct {
	SheetYear int
	relatedLink
}

// relatedGroup は関連性ごとにまとめた関連事業（シートの出現順）。
type relatedGroup struct {
	Kind  string
	Items []relatedLink
}

func relatedGroups(rels []rs.Related, known map[string]bool, root string) []relatedGroup {
	var groups []relatedGroup
	index := map[string]int{}
	for _, r := range rels {
		kind := r.Kind
		if kind == "" {
			kind = "関連性の記載なし"
		}
		i, ok := index[kind]
		if !ok {
			i = len(groups)
			index[kind] = i
			groups = append(groups, relatedGroup{Kind: kind})
		}
		groups[i].Items = append(groups[i].Items, relatedLinkOf(r, known, root))
	}
	return groups
}

func relatedLinkOf(r rs.Related, known map[string]bool, root string) relatedLink {
	l := relatedLink{ID: r.ID, Name: r.Name}
	if known[r.ID] && idPattern.MatchString(r.ID) {
		l.Href = root + "p/" + r.ID + ".html"
	}
	return l
}

// writeDetail は 1 事業の詳細ページを書く。th は兆候の根拠の文言に使う（判定は sm で済んでいる）。
// known はページのある予算事業ID の集合で、関連事業をリンクにするかどうかに使う（nil なら名前だけ）。
func writeDetail(w io.Writer, tl *lifecycle.Timeline, sm lifecycle.Summary, th lifecycle.Thresholds, known map[string]bool, generated string) error {
	lc := tl.Latest
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
	wfIn, wfOut, wfNote := waterfall(lc)
	const root = "../"
	var parents []relatedLink
	var past []pastParent
	if lc.AllZero {
		for _, p := range lc.Parents() {
			parents = append(parents, relatedLinkOf(p, known, root))
		}
		for _, p := range tl.PastParents {
			past = append(past, pastParent{SheetYear: p.SheetYear, relatedLink: relatedLinkOf(p.Related, known, root)})
		}
	}
	return templates.ExecuteTemplate(w, "detail.html", detailData{
		LC: lc, TL: tl, Multi: len(tl.SheetYears) > 1, Summary: sm,
		Digest: digest(tl), Flow: flowSteps(lc, sm), WfIn: wfIn, WfOut: wfOut, WfNote: wfNote,
		YearBars: ybars, Badges: signalBadges(tl, sm.Signals, th),
		Related: relatedGroups(lc.Related, known, root), Parents: parents, PastParents: past,
		Generated: generated, Root: root,
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

// nonZero は JSON 用に空欄と 0 を nil にする。
func nonZero(v rs.Yen) *int64 {
	if v.Value == 0 {
		return nil
	}
	return yenPtr(v)
}

// yenPtr は JSON 用に空欄を nil にする。
func yenPtr(v rs.Yen) *int64 {
	if !v.Valid {
		return nil
	}
	x := v.Value
	return &x
}
