// Package render は組み立て済みの lifecycle.Lifecycle を人が読む形で書き出す。
// 列名や CSV の知識は持たない。
package render

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/rs"
)

// Attribution は出力に必ず含める出典表記。公共データ利用規約（PDL1.0）の求める
// 出典・加工したこと・加工の主体と、推奨されるデータの URL・規約名を入れる。
// HTML では internal/site/templates/partials.html の "credit" が同じ文言をリンク付きで出す。
const Attribution = "出典: 行政事業レビュー見える化サイトのデータを加工して作成（加工: zailoop / kwrkb）。データ: https://rssystem.go.jp/ 、公共データ利用規約 第1.0版（PDL1.0）"

// Disclaimer は出典と並べて出す注記。加工した情報を国が作成したかのように見せないため
// （見える化サイトの利用規約）、非公式であることと、独自の判定であることを明示する。
const Disclaimer = "zailoop は非公式のツールで、国や府省庁が作成したものではありません。兆候とループ検証は zailoop 独自の判定です。"

// Text は 1 事業のライフサイクルをテキストで書き出す。
func Text(w io.Writer, lc *lifecycle.Lifecycle) error {
	p := &printer{w: w}
	pr := lc.Project
	p.f("%s  %s", pr.ID, pr.Name)
	p.f("%s", strings.TrimSpace(strings.Join(nonEmpty(pr.Ministry, pr.Bureau, pr.Division), " / ")))
	meta := nonEmpty(pr.Category, yearsLabel("開始", pr.StartYear), yearsLabel("終了予定", pr.EndYear))
	if len(pr.MajorExpense) > 0 {
		meta = append(meta, "主要経費: "+strings.Join(pr.MajorExpense, "、"))
	}
	if len(pr.Methods) > 0 {
		meta = append(meta, "実施方法: "+strings.Join(pr.Methods, "、"))
	}
	p.f("%s", strings.Join(meta, " | "))
	p.f("%d年度シート（予算年度 %s）  軸: %d年度執行実績", lc.SheetYear, intsJoin(lc.BudgetYears), lc.ActualYear)
	p.f("")

	n := lc.ActualYear
	// ① 要求
	p.h("① 要求", fmt.Sprintf("FY%d 概算要求", n))
	p.kv("要求額", amount(lc.Request.Amount, lc.Request.State))

	// ② 成立
	e := lc.Enacted
	p.h("② 成立", fmt.Sprintf("FY%d 予算", n))
	if e.State == lifecycle.StateOK {
		p.kv("当初予算", Yen(e.Initial))
		p.kv("補正予算", Yen(e.Supplementary))
		p.kv("前年度繰越", Yen(e.CarriedIn))
		p.kv("予備費等", Yen(e.Reserve))
		p.kv("歳出予算現額", Yen(e.Current))
		if len(e.Items) > 0 {
			var parts []string
			for _, it := range e.Items {
				parts = append(parts, fmt.Sprintf("%s %s（%d件）", it.Kind, Yen(it.Amount), it.Count))
			}
			p.kv("内訳(2-2)", strings.Join(parts, "、"))
		}
	} else {
		p.kv("予算", e.State.String())
	}

	// ③ 執行
	x := lc.Execution
	p.h("③ 執行", fmt.Sprintf("FY%d 執行実績", n))
	switch x.State {
	case lifecycle.StateOK:
		p.kv("執行額", Yen(x.Executed))
		p.kv("執行率", Ratio(x.Rate))
	case lifecycle.StateNotComputable:
		p.kv("執行額", Yen(x.Executed)+"（歳出予算現額が 0 以下。予算は別事業に計上の可能性）")
		p.kv("執行率", "—（算出対象外）")
	default:
		p.kv("執行", x.State.String())
	}

	// ④ 決算
	st := lc.Settlement
	p.h("④ 決算", fmt.Sprintf("FY%d 繰越・不用（不用相当額は 現額−執行−翌年度繰越 の導出値）", n))
	p.kv("翌年度繰越", Yen(st.CarriedOut))
	switch st.UnusedState {
	case lifecycle.StateOK:
		p.kv("不用相当額", Yen(st.Unused))
	case lifecycle.StateNeedsReview:
		p.kv("不用相当額", fmt.Sprintf("算出不能／差額 %s（要確認）", YenValue(st.Diff)))
	case lifecycle.StateNotComputable:
		p.kv("不用相当額", "—（算出対象外）")
	default:
		p.kv("不用相当額", st.UnusedState.String())
	}

	// ⑤ 評価
	ev := lc.Evaluation
	p.h("⑤ 評価", fmt.Sprintf("%d年度シートの点検・評価", lc.SheetYear))
	p.kv("所管部局の点検", or(ev.SelfCheck, "（記載なし）"))
	p.kv("改善の方向性", or(ev.Improvement, "（記載なし）"))
	ext := or(ev.ExternalTarget, "（記載なし）")
	if ev.ExternalYear != "" {
		ext += fmt.Sprintf("（最終実施 %s年度）", ev.ExternalYear)
	}
	p.kv("外部有識者点検", ext)
	if ev.ExternalOpinion != "" {
		p.kv("  所見", ev.ExternalOpinion)
	}
	p.kv("推進チーム所見", or(ev.TeamOpinion, "（記載なし）"))
	if ev.TeamOpinionDetail != "" {
		p.kv("  詳細", ev.TeamOpinionDetail)
	}
	if len(ev.Outcomes) > 0 {
		p.kv(fmt.Sprintf("成果指標(FY%d)", n), "")
		for _, o := range ev.Outcomes {
			p.f("    - [%s%s] %s: 目標 %s / 実績 %s / 達成率 %s", o.Kind, TermLabel(o.Term), MetricLabel(o), or(o.Target, "—"), or(o.Actual, "—"), or(o.Rate, "—"))
		}
	}

	// ⑥ 翌年度反映
	r := lc.Reflection
	p.h("⑥ 翌年度反映", fmt.Sprintf("概算要求への反映と FY%d 要求", r.RequestYear))
	p.kv("反映状況", or(r.Status, "（記載なし）"))
	if r.Detail != "" {
		p.kv("  詳細", r.Detail)
	}
	if r.ReflectedGeneral.Valid {
		p.kv("反映額(一般会計)", Yen(r.ReflectedGeneral))
	}
	for _, sp := range r.ReflectedSpecial {
		p.kv("反映額(特別会計)", fmt.Sprintf("%s %s %s", sp.Account, sp.Subaccount, Yen(sp.Amount)))
	}
	p.kv(fmt.Sprintf("FY%d 当初予算（参考）", n+1), amount(r.NextInitial, r.NextInitialState))
	p.kv(fmt.Sprintf("FY%d 概算要求", r.RequestYear), amount(r.Amount, r.State))
	if r.Demand.Valid && r.Demand.Value != 0 {
		p.kv("  うち要望額", Yen(r.Demand))
	}
	if r.ChangeReason != "" {
		p.kv("  主な増減理由", r.ChangeReason)
	}

	// 支出先
	if lc.BlockCount > 0 {
		p.h("支出先ブロック", fmt.Sprintf("FY%d 実績、金額上位 %d / %d ブロック（ブロック間の資金移動があるため合計は事業総額ではない）", n, len(lc.Payees), lc.BlockCount))
		for _, b := range lc.Payees {
			line := fmt.Sprintf("%s %s", b.Number, b.Name)
			if b.Role != "" {
				line += "（" + b.Role + "）"
			}
			p.kv(line, Yen(b.Total))
		}
	}

	if len(lc.Notes) > 0 {
		p.h("注意", "")
		for _, nt := range lc.Notes {
			p.f("  - %s", nt)
		}
	}
	p.f("")
	p.f("%s", Attribution)
	p.f("%s", Disclaimer)
	return p.err
}

type printer struct {
	w   io.Writer
	err error
}

func (p *printer) f(format string, a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format+"\n", a...)
}

func (p *printer) h(title, sub string) {
	p.f("")
	if sub != "" {
		p.f("%s  %s", title, sub)
	} else {
		p.f("%s", title)
	}
}

func (p *printer) kv(k, v string) {
	if v == "" {
		p.f("  %s", k)
		return
	}
	p.f("  %-18s %s", k, v)
}

func amount(v rs.Yen, st lifecycle.State) string {
	if st != lifecycle.StateOK {
		return st.String()
	}
	return Yen(v)
}

// Yen は円を 3 桁区切りで表す。空欄は「—」。
func Yen(v rs.Yen) string {
	if !v.Valid {
		return "—"
	}
	return YenValue(v.Value)
}

// YenValue は int64 の円を 3 桁区切りで表す。
func YenValue(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprint(v)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String() + " 円"
	}
	return b.String() + " 円"
}

// Ratio は執行率などを百分率で表す。空欄は「—」。
func Ratio(r rs.Ratio) string {
	if !r.Valid {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", r.Value*100)
}

func or(s, alt string) string {
	if s == "" {
		return alt
	}
	return s
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func yearsLabel(label, year string) string {
	if year == "" {
		return ""
	}
	return label + " " + year + "年度"
}

func intsJoin(xs []int) string {
	if len(xs) == 0 {
		return "なし"
	}
	if len(xs) == 1 {
		return fmt.Sprint(xs[0])
	}
	return fmt.Sprintf("%d〜%d", xs[0], xs[len(xs)-1])
}

// TermLabel はアウトカムの期間を「・短期」のように表す。
func TermLabel(term string) string {
	if term == "" {
		return ""
	}
	return "・" + strings.TrimLeft(term, "0123456789.")
}

// MetricLabel は指標名（無ければ目標）に単位を添える。
func MetricLabel(o lifecycle.OutcomeLine) string {
	s := o.Metric
	if s == "" {
		s = o.Goal
	}
	if o.Unit != "" {
		s += "[" + o.Unit + "]"
	}
	return s
}

// YenShort は円を兆・億・万で短く表す（web/src/format.ts の yenShort と同じ規則）。
// 3 桁以上は整数に丸めて 3 桁区切り、それ未満は小数 1 桁（末尾の .0 は落とす）。空欄は「—」。
func YenShort(v rs.Yen) string {
	if !v.Valid {
		return "—"
	}
	return YenShortValue(v.Value)
}

// YenShortValue は int64 の円を YenShort と同じ規則で表す。
func YenShortValue(v int64) string {
	neg := v < 0
	a := v
	if neg {
		a = -v
	}
	var s string
	switch {
	case a >= 1e12:
		s = shortNum(float64(a)/1e12) + " 兆円"
	case a >= 1e8:
		s = shortNum(float64(a)/1e8) + " 億円"
	case a >= 1e4:
		s = shortNum(float64(a)/1e4) + " 万円"
	default:
		s = fmt.Sprint(a) + " 円"
	}
	if neg {
		return "-" + s
	}
	return s
}

func shortNum(x float64) string {
	if x >= 100 {
		return strings.TrimSuffix(YenValue(int64(math.Floor(x+0.5))), " 円")
	}
	return strconv.FormatFloat(math.Floor(x*10+0.5)/10, 'f', -1, 64)
}
