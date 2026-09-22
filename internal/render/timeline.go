package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/kwrkb/zailoop/internal/lifecycle"
)

// Timeline は複数年度シートを束ねた Timeline を書き出す。
// 最新シートの 6 段階（Text）に続けて、「年度をまたぐ推移」と「ループ検証」を出す。
func Timeline(w io.Writer, tl *lifecycle.Timeline) error {
	if err := Text(w, tl.Latest); err != nil {
		return err
	}
	p := &printer{w: w}
	p.f("")
	p.f("==== 年度をまたぐ推移  シート: %s（重なる年度は新しいシートの値）", intsList(tl.SheetYears))
	if len(tl.Names) > 1 {
		var parts []string
		for _, n := range tl.Names {
			parts = append(parts, fmt.Sprintf("%d年度〜「%s」", n.SheetYear, n.Name))
		}
		p.f("  事業名の変遷: %s", strings.Join(parts, " → "))
	}
	p.f("  %-6s %-18s %-18s %-18s %-18s %-8s %-18s %s", "FY", "要求", "当初", "現額", "執行", "執行率", "不用相当", "出典")
	for _, y := range tl.Years {
		exec := "未確定"
		rate := "—"
		if y.ExecState == lifecycle.StateOK {
			exec = Yen(y.Executed)
			rate = Ratio(y.ExecRate)
		} else if y.ExecState == lifecycle.StateNotComputable {
			exec = Yen(y.Executed) + "*"
		}
		unused := "—"
		switch y.UnusedState {
		case lifecycle.StateOK:
			unused = Yen(y.Unused)
		case lifecycle.StateNeedsReview:
			unused = "要確認"
		}
		src := fmt.Sprintf("%dシート", y.Source)
		if len(y.Revised) > 0 {
			src += "（改訂）"
		}
		p.f("  %-6d %-18s %-18s %-18s %-18s %-8s %-18s %s", y.FY, Yen(y.Request), Yen(y.Initial), Yen(y.Current), exec, rate, unused, src)
	}
	p.f("")
	p.f("==== ループ検証  シートの評価・反映 → 翌年の成立・執行")
	for _, lp := range tl.Loops {
		p.f("  %d年度シート: 所見「%s」 反映「%s」%s", lp.SheetYear, or(lp.TeamOpinion, "—"), or(lp.Reflection, "—"), reflected(lp))
		p.f("    FY%d 当初 %s → FY%d 要求 %s", lp.SheetYear, Yen(lp.Initial), lp.SheetYear+1, Yen(lp.NextRequest))
		if !lp.Closed {
			p.f("    → 翌年のシートがないため未検証")
			continue
		}
		p.f("    → FY%d 当初 %s（%d年度シート）  FY%d 執行 %s（%s）", lp.SheetYear+1, Yen(lp.NextInitial), lp.SheetYear+1, lp.SheetYear, Yen(lp.Executed), Ratio(lp.ExecRate))
		p.f("    判定: %s%s", lp.Verdict, verdictNote(lp))
	}
	if len(tl.Notes) > 0 {
		p.f("")
		p.f("==== 注意（年度間の差異）")
		for _, n := range tl.Notes {
			p.f("  - %s", n)
		}
	}
	return p.err
}

func reflected(lp lifecycle.Loop) string {
	if lp.ReflectedGeneral.Valid {
		return " 反映額 " + Yen(lp.ReflectedGeneral)
	}
	return ""
}

func verdictNote(lp lifecycle.Loop) string {
	switch lp.Verdict {
	case lifecycle.VerdictContradiction:
		return "（縮減・廃止・終了予定なのに翌年当初が増加）"
	case lifecycle.VerdictConsistent:
		return "（翌年当初は減少または横ばい）"
	case lifecycle.VerdictNeutral:
		return "（反映状況は増減を約束していない）"
	}
	return ""
}

func intsList(xs []int) string {
	var s []string
	for _, x := range xs {
		s = append(s, fmt.Sprint(x))
	}
	return strings.Join(s, ", ")
}
