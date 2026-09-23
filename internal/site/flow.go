package site

import (
	"fmt"
	"math"
	"strings"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/render"
	"github.com/kwrkb/zailoop/internal/rs"
)

// 詳細ページ上部の「流れ」「お金の内訳」「要点」「兆候の根拠」を組み立てる。
// 判定はしない（Signal / Verdict は lifecycle で済んでいる）。ここでは値を並べて文言にするだけ。

// flowStep は ①〜⑥ の 1 段。Money が false の段（評価・反映）は文言が主役になる。
type flowStep struct {
	Anchor string // 下の詳細セクションへのリンク先
	No     string
	Title  string
	Period string // 「FY2023 概算要求」など
	Main   string // 大きく出す値（短い金額 or 文言）
	Full   string // Main の全桁（title 属性）
	Subs   []string
	Money  bool
}

func flowSteps(lc *lifecycle.Lifecycle, sm lifecycle.Summary) []flowStep {
	n := lc.ActualYear
	short := func(v rs.Yen) (string, string) { return render.YenShort(v), render.Yen(v) }
	steps := make([]flowStep, 0, 6)

	req := flowStep{Anchor: "s1", No: "①", Title: "要求", Period: fmt.Sprintf("FY%d 概算要求", n), Money: true}
	if lc.Request.State == lifecycle.StateOK {
		req.Main, req.Full = short(lc.Request.Amount)
	} else {
		req.Main = lc.Request.State.String()
	}
	steps = append(steps, req)

	en := flowStep{Anchor: "s2", No: "②", Title: "成立", Period: fmt.Sprintf("FY%d 当初予算", n), Money: true}
	if e := lc.Enacted; e.State == lifecycle.StateOK {
		en.Main, en.Full = short(e.Initial)
		if r, ok := ratio(e.Initial, sm.Request); ok {
			en.Subs = append(en.Subs, "要求の "+pct(r))
		}
		if e.Current.Valid && e.Current != e.Initial {
			en.Subs = append(en.Subs, "現額 "+render.YenShort(e.Current))
		}
	} else {
		en.Main = e.State.String()
	}
	steps = append(steps, en)

	ex := flowStep{Anchor: "s3", No: "③", Title: "執行", Period: fmt.Sprintf("FY%d 執行額", n), Money: true}
	switch x := lc.Execution; x.State {
	case lifecycle.StateOK:
		ex.Main, ex.Full = short(x.Executed)
		ex.Subs = append(ex.Subs, "執行率 "+render.Ratio(x.Rate))
	case lifecycle.StateNotComputable:
		ex.Main, ex.Full = short(x.Executed)
		ex.Subs = append(ex.Subs, "現額 0 以下（算出対象外）")
	default:
		ex.Main = x.State.String()
	}
	steps = append(steps, ex)

	st := flowStep{Anchor: "s4", No: "④", Title: "決算", Period: fmt.Sprintf("FY%d 不用相当額", n), Money: true}
	switch s := lc.Settlement; s.UnusedState {
	case lifecycle.StateOK:
		st.Main, st.Full = short(s.Unused)
		if r, ok := ratio(s.Unused, lc.Enacted.Current); ok {
			st.Subs = append(st.Subs, "現額の "+pct(r))
		}
	case lifecycle.StateNeedsReview:
		st.Main = "要確認"
		st.Subs = append(st.Subs, "差額 "+render.YenShortValue(s.Diff))
	default:
		st.Main = s.UnusedState.String()
	}
	if co := lc.Settlement.CarriedOut; co.Valid && co.Value != 0 {
		st.Subs = append(st.Subs, "翌年度繰越 "+render.YenShort(co))
	}
	steps = append(steps, st)

	ev := flowStep{Anchor: "s5", No: "⑤", Title: "評価", Period: fmt.Sprintf("%d年度シート 推進チーム所見", lc.SheetYear)}
	ev.Main = orText(lc.Evaluation.TeamOpinion, "記載なし")
	if t := outcomeSummary(lc); t != "" {
		ev.Subs = append(ev.Subs, t)
	}
	steps = append(steps, ev)

	rf := flowStep{Anchor: "s6", No: "⑥", Title: "反映", Period: fmt.Sprintf("FY%d 要求への反映", lc.Reflection.RequestYear)}
	rf.Main = orText(lc.Reflection.Status, "記載なし")
	if r := lc.Reflection; r.State == lifecycle.StateOK {
		rf.Subs = append(rf.Subs, fmt.Sprintf("FY%d 要求 %s", r.RequestYear, render.YenShort(r.Amount)))
	}
	steps = append(steps, rf)
	return steps
}

// outcomeSummary は成果指標の達成率の範囲を 1 行にする（アウトカムの定量指標だけ）。
func outcomeSummary(lc *lifecycle.Lifecycle) string {
	rates, outcomes := lifecycle.OutcomeRates(lc)
	if len(rates) == 0 {
		if outcomes > 0 {
			return fmt.Sprintf("成果指標 %d 件（達成率なし）", outcomes)
		}
		return ""
	}
	mn, mx := rates[0], rates[0]
	for _, r := range rates[1:] {
		mn, mx = min(mn, r), max(mx, r)
	}
	switch {
	case len(rates) == 1:
		return fmt.Sprintf("成果達成率 %s%%", trimFloat(mn))
	case mn == mx:
		return fmt.Sprintf("成果達成率 %s%%（%d 件）", trimFloat(mn), len(rates))
	}
	return fmt.Sprintf("成果達成率 %s〜%s%%（%d 件）", trimFloat(mn), trimFloat(mx), len(rates))
}

// wfRow は「お金の内訳」の 1 行。X と W は 0〜100 の百分率（全行で共通の尺度）。
type wfRow struct {
	Label string
	Text  string // 短い金額
	Full  string // 全桁
	X, W  float64
	Class string // in / total / exec / carry / unused / neg
}

// waterfall は歳出予算現額の入口（当初＋補正＋繰越＋予備費等）と出口（執行＋翌年度繰越＋不用相当額）を
// 共通の尺度で並べる。入口の和は現額と一致する（2024・2025 年度の全事業で確認済み）。
// 0 の項目は省き、負の補正・予備費等は減る段として描く。現額が 0 以下なら描かない。
func waterfall(lc *lifecycle.Lifecycle) (in, out []wfRow, note string) {
	e := lc.Enacted
	if e.State != lifecycle.StateOK || !e.Current.Valid || e.Current.Value <= 0 {
		return nil, nil, ""
	}
	type step struct {
		label, class string
		v            rs.Yen
		start, end   int64
	}
	var steps []step
	var cum int64
	add := func(label, class string, v rs.Yen, keepZero bool) {
		if !v.Valid || (v.Value == 0 && !keepZero) {
			return
		}
		steps = append(steps, step{label: label, class: class, v: v, start: cum, end: cum + v.Value})
		cum += v.Value
	}
	add("当初予算", "in", e.Initial, true)
	add("補正予算", "in", e.Supplementary, false)
	add("前年度繰越", "in", e.CarriedIn, false)
	add("予備費等", "in", e.Reserve, false)
	nIn := len(steps)
	steps = append(steps, step{label: "歳出予算現額", class: "total", v: e.Current, start: 0, end: e.Current.Value})

	if lc.Execution.State == lifecycle.StateOK {
		cum = 0
		add("執行額", "exec", lc.Execution.Executed, true)
		add("翌年度繰越", "carry", lc.Settlement.CarriedOut, false)
		switch lc.Settlement.UnusedState {
		case lifecycle.StateOK:
			add("不用相当額", "unused", lc.Settlement.Unused, false)
		case lifecycle.StateNeedsReview:
			note = fmt.Sprintf("執行額＋翌年度繰越が歳出予算現額を %s 上回るため、不用相当額は算出していません（要確認）。", render.YenShortValue(-lc.Settlement.Diff))
		}
	}

	var top int64
	for _, s := range steps {
		top = max(top, s.start, s.end)
	}
	pctOf := func(v int64) float64 { return float64(v) / float64(top) * 100 }
	for i, s := range steps {
		lo, hi := min(s.start, s.end), max(s.start, s.end)
		r := wfRow{Label: s.label, Text: render.YenShort(s.v), Full: render.Yen(s.v), X: pctOf(lo), W: pctOf(hi - lo), Class: s.class}
		if s.v.Value < 0 {
			r.Class = "neg"
		}
		if i <= nIn {
			in = append(in, r)
		} else {
			out = append(out, r)
		}
	}
	return in, out, note
}

// digest は詳細ページ冒頭の要点（2〜4 文）。値と、lifecycle が済ませた判定だけで作り、
// 形容（大幅・適切 など）は足さない。年度は必ず FY / シート年度を添える。
func digest(tl *lifecycle.Timeline) []string {
	lc := tl.Latest
	n := lc.ActualYear
	var out []string

	e, x, st := lc.Enacted, lc.Execution, lc.Settlement
	switch {
	case e.State != lifecycle.StateOK:
		out = append(out, fmt.Sprintf("FY%d の予算額はシートにありません（%s。新規事業など）。", n, e.State))
	case x.State == lifecycle.StateNotComputable:
		out = append(out, fmt.Sprintf("FY%d は歳出予算現額が 0 以下で、執行額は %s（予算は別事業に計上されている可能性）。", n, render.YenShort(x.Executed)))
	case x.State == lifecycle.StateOK:
		s := fmt.Sprintf("FY%d は歳出予算現額 %s のうち %s（%s）を執行", n, render.YenShort(e.Current), render.YenShort(x.Executed), render.Ratio(x.Rate))
		var rest []string
		if st.CarriedOut.Valid && st.CarriedOut.Value != 0 {
			rest = append(rest, "翌年度繰越 "+render.YenShort(st.CarriedOut))
		}
		switch st.UnusedState {
		case lifecycle.StateOK:
			rest = append(rest, "不用相当額 "+render.YenShort(st.Unused))
		case lifecycle.StateNeedsReview:
			rest = append(rest, "不用相当額は算出不能（要確認）")
		}
		if len(rest) > 0 {
			s += "。" + strings.Join(rest, "、")
		}
		out = append(out, s+"。")
	default:
		out = append(out, fmt.Sprintf("FY%d の歳出予算現額は %s、執行は%s。", n, render.YenShort(e.Current), x.State))
	}

	ev, rf := lc.Evaluation, lc.Reflection
	if ev.TeamOpinion != "" || rf.Status != "" {
		out = append(out, fmt.Sprintf("%d年度シートの推進チーム所見は「%s」、翌年度要求への反映は「%s」。", lc.SheetYear, orText(ev.TeamOpinion, "記載なし"), orText(rf.Status, "記載なし")))
	}

	// ループ検証の文があるときは、増減はそちら（判定と同じ基準額）だけで示す。
	// シート間で FY N 当初が改訂された事業で、隣り合う 2 文の基準額が食い違わないようにする。
	var loop string
	if len(tl.Loops) >= 2 {
		lp := tl.Loops[len(tl.Loops)-2]
		if lp.Closed && lp.Verdict != lifecycle.VerdictUnknown {
			loop = fmt.Sprintf("ループ検証（このサイト独自の判定）: %d年度シートの反映「%s」に対し、FY%d 当初は %s → FY%d 当初 %s", lp.SheetYear, orText(lp.Reflection, "記載なし"), lp.SheetYear, render.YenShort(lp.Initial), lp.SheetYear+1, render.YenShort(lp.NextInitial))
			if d, ok := change(lp.Initial, lp.NextInitial); ok {
				loop += "（" + d + "）"
			}
			if lp.Verdict == lifecycle.VerdictNeutral {
				loop += "。この反映状況は増減を約束しないため判定対象外。"
			} else {
				loop += fmt.Sprintf("。判定は「%s」。", lp.Verdict)
				if lp.Verdict == lifecycle.VerdictContradiction {
					loop += render.ContradictionNote(lp.Reflection)
				}
			}
		}
	}

	var next []string
	if rf.NextInitialState == lifecycle.StateOK {
		s := fmt.Sprintf("FY%d 当初予算は %s", n+1, render.YenShort(rf.NextInitial))
		if d, ok := change(e.Initial, rf.NextInitial); ok && e.State == lifecycle.StateOK && loop == "" {
			s += fmt.Sprintf("（FY%d 当初比 %s）", n, d)
		}
		next = append(next, s)
	}
	if rf.State == lifecycle.StateOK {
		next = append(next, fmt.Sprintf("FY%d 概算要求は %s", rf.RequestYear, render.YenShort(rf.Amount)))
	}
	if len(next) > 0 {
		out = append(out, strings.Join(next, "、")+"。")
	}

	if loop != "" {
		out = append(out, loop)
	}
	return out
}

// badge は兆候 1 つ分。Evidence は判定に使った値の文言（なければ空）。
type badge struct {
	Label       string
	Description string
	Evidence    string
}

func signalBadges(tl *lifecycle.Timeline, s lifecycle.Signal, th lifecycle.Thresholds) []badge {
	desc := map[lifecycle.Signal]lifecycle.SignalDesc{}
	for _, d := range lifecycle.SignalInfo {
		desc[d.Signal] = d
	}
	var out []badge
	for _, ev := range lifecycle.Explain(tl, s, th) {
		d := desc[ev.Signal]
		out = append(out, badge{Label: d.Label, Description: d.Description, Evidence: evidenceText(ev)})
	}
	return out
}

func evidenceText(ev lifecycle.Evidence) string {
	switch ev.Signal {
	case lifecycle.SignalRequestGap:
		return fmt.Sprintf("当初は要求の %s（基準 %s 未満）", pct(ev.Observed), pct(ev.Threshold))
	case lifecycle.SignalLowExecution:
		return fmt.Sprintf("執行率 %s（基準 %s 未満）", pct(ev.Observed), pct(ev.Threshold))
	case lifecycle.SignalLargeUnused:
		return fmt.Sprintf("%s・現額の %s（基準 %s 以上かつ %s 以上）", render.YenShort(ev.Amount), pct(ev.Observed), render.YenShortValue(ev.ThresholdAmount), pct(ev.Threshold))
	case lifecycle.SignalCut:
		return "反映状況「" + ev.Text + "」"
	case lifecycle.SignalExecutionWithoutBudget:
		return fmt.Sprintf("現額 %s で執行 %s", render.YenShort(ev.Base), render.YenShort(ev.Amount))
	case lifecycle.SignalNegativeUnused:
		return "差額 " + render.YenShort(ev.Amount)
	case lifecycle.SignalOutcomeShortfall:
		return fmt.Sprintf("達成率の最小 %s%%（基準 %s%% 未満）", trimFloat(ev.Observed), trimFloat(ev.Threshold))
	case lifecycle.SignalOutcomeOvershoot:
		return fmt.Sprintf("達成率の最大 %s%%（基準 %s%% 超）", trimFloat(ev.Observed), trimFloat(ev.Threshold))
	case lifecycle.SignalReflectionContradicted:
		return fmt.Sprintf("%d年度シートの反映「%s」、FY%d 当初 %s → FY%d 当初 %s", ev.Year, ev.Text, ev.Year, render.YenShort(ev.Base), ev.Year+1, render.YenShort(ev.Amount))
	case lifecycle.SignalRequestZeroed:
		return fmt.Sprintf("FY%d 要求 %s → FY%d 当初 %s", ev.Year+1, render.YenShort(ev.Base), ev.Year+1, render.YenShort(ev.Amount))
	}
	return ""
}

// ratio は a/b。b が 0 以下か値がなければ false。
func ratio(a, b rs.Yen) (float64, bool) {
	if !a.Valid || !b.Valid || b.Value <= 0 {
		return 0, false
	}
	return float64(a.Value) / float64(b.Value), true
}

// change は from → to の増減率を「+8.0%」「−12.3%」「増減なし」で表す。from が 0 以下なら false。
func change(from, to rs.Yen) (string, bool) {
	r, ok := ratio(to, from)
	if !ok {
		return "", false
	}
	d := (r - 1) * 100
	switch {
	case math.Abs(d) < 0.05:
		if to.Value == from.Value {
			return "増減なし", true
		}
		if d < 0 {
			return "−0.0%", true
		}
		return "+0.0%", true
	case d < 0:
		return fmt.Sprintf("−%.1f%%", -d), true
	}
	return fmt.Sprintf("+%.1f%%", d), true
}

func pct(r float64) string { return trimFloat(math.Round(r*1000)/10) + "%" }

func trimFloat(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	return strings.TrimSuffix(s, ".0")
}

func orText(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}
