package lifecycle

import (
	"fmt"
	"sort"

	"github.com/kwrkb/zailoop/internal/rs"
)

// NameAt は事業年度ごとの事業名（名称変更の履歴）。
type NameAt struct {
	SheetYear int
	Name      string
}

// YearRow は予算年度 1 年分。値は「その予算年度を含む最新のシート」から取る。
type YearRow struct {
	FY            int
	Request       rs.Yen // FY−1 の行の翌年度要求額（= FY の概算要求）
	Initial       rs.Yen
	Supplementary rs.Yen
	CarriedIn     rs.Yen
	Reserve       rs.Yen
	Current       rs.Yen
	Executed      rs.Yen
	ExecRate      rs.Ratio
	ExecState     State
	CarriedOut    rs.Yen
	Unused        rs.Yen
	UnusedState   State
	Source        int      // 値を取ったシートの事業年度
	Revised       []string // 古いシートと違った項目の説明
}

// LoopVerdict はシート S の「反映状況」と、翌年のシートで分かった FY S+1 当初予算の整合。
type LoopVerdict int

const (
	VerdictUnknown       LoopVerdict = iota // 翌年のシートがない、または当初予算が比較できない
	VerdictNeutral                          // 反映状況が増減を約束していない（現状通り・執行等改善 など）
	VerdictConsistent                       // 縮減・廃止・終了予定 → 翌年当初が減った（または 0）
	VerdictContradiction                    // 縮減・廃止・終了予定 → 翌年当初が増えた
)

func (v LoopVerdict) String() string {
	switch v {
	case VerdictNeutral:
		return "判定対象外"
	case VerdictConsistent:
		return "整合"
	case VerdictContradiction:
		return "矛盾"
	}
	return "不明"
}

// Loop はシート S の評価・反映が翌年にどうなったか。
type Loop struct {
	SheetYear        int    // S
	TeamOpinion      string // S の推進チーム所見
	Reflection       string // S の反映状況
	ReflectedGeneral rs.Yen // S の反映額（一般会計）
	Initial          rs.Yen // FY S 当初（S シート）
	NextRequest      rs.Yen // FY S+1 概算要求（S シート）
	NextInitial      rs.Yen // FY S+1 当初（S+1 シート）。Closed のときだけ有効
	Executed         rs.Yen // FY S 執行（S+1 シートで確定）
	ExecRate         rs.Ratio
	Closed           bool // S+1 のシートがある
	Verdict          LoopVerdict
	Signals          Signal
}

// Timeline は 1 事業の複数年度シートを束ねたもの。
type Timeline struct {
	ID         string
	Names      []NameAt
	SheetYears []int      // 手元にあるシートの事業年度（昇順）
	Latest     *Lifecycle // 最新シートの Lifecycle
	Years      []YearRow  // 予算年度の昇順
	Loops      []Loop     // シートごと（昇順）
	Notes      []string
}

// Track は同じ事業の複数年度シート（事業年度の昇順でなくてよい）から Timeline を作る。
// 重なる予算年度の値は新しいシートを正とし、古いシートとの差異は Revised と Notes に残す。
func Track(sheets []*rs.Sheet, th Thresholds) *Timeline {
	if len(sheets) == 0 {
		return nil
	}
	ss := append([]*rs.Sheet(nil), sheets...)
	sort.SliceStable(ss, func(i, j int) bool { return ss[i].FiscalYear < ss[j].FiscalYear })
	tl := &Timeline{ID: ss[len(ss)-1].Project.ID}
	lcs := make([]*Lifecycle, len(ss))
	for i, s := range ss {
		lcs[i] = Build(s, Options{})
		tl.SheetYears = append(tl.SheetYears, s.FiscalYear)
		if len(tl.Names) == 0 || tl.Names[len(tl.Names)-1].Name != s.Project.Name {
			tl.Names = append(tl.Names, NameAt{SheetYear: s.FiscalYear, Name: s.Project.Name})
		}
		if s.Project.ID != tl.ID {
			tl.Notes = append(tl.Notes, fmt.Sprintf("事業年度%d のシートは予算事業ID %s（%s と異なる）", s.FiscalYear, s.Project.ID, tl.ID))
		}
	}
	tl.Latest = lcs[len(lcs)-1]
	tl.Years = trackYears(ss, tl)
	tl.Loops = trackLoops(ss, lcs, th)
	return tl
}

func trackYears(ss []*rs.Sheet, tl *Timeline) []YearRow {
	// 予算年度の和集合
	fys := map[int]bool{}
	for _, s := range ss {
		for _, b := range s.Budgets {
			if b.HasTotal {
				fys[b.Year] = true
			}
		}
	}
	var years []int
	for y := range fys {
		years = append(years, y)
	}
	sort.Ints(years)
	var rows []YearRow
	for _, fy := range years {
		row := YearRow{FY: fy, ExecState: StateMissing, UnusedState: StateMissing}
		// 最新シートから順に探す
		var newest *rs.Sheet
		for i := len(ss) - 1; i >= 0; i-- {
			if b := ss[i].Budget(fy); b != nil && b.HasTotal {
				if newest == nil {
					newest = ss[i]
					row.Source = ss[i].FiscalYear
					lc := Build(ss[i], Options{ActualYear: fy})
					row.Initial, row.Supplementary, row.CarriedIn, row.Reserve, row.Current = lc.Enacted.Initial, lc.Enacted.Supplementary, lc.Enacted.CarriedIn, lc.Enacted.Reserve, lc.Enacted.Current
					row.Executed, row.ExecRate, row.ExecState = lc.Execution.Executed, lc.Execution.Rate, lc.Execution.State
					row.CarriedOut, row.Unused, row.UnusedState = lc.Settlement.CarriedOut, lc.Settlement.Unused, lc.Settlement.UnusedState
					if fy > ss[i].FiscalYear-1 {
						// 事業年度以降の予算年度は執行が未確定
						row.ExecState, row.UnusedState = StateMissing, StateMissing
						row.Executed, row.Unused = rs.Yen{}, rs.Yen{}
					}
				} else {
					// 古いシートとの差異
					nb := newest.Budget(fy).Total
					for _, d := range []struct {
						name   string
						o, n   rs.Yen
						unless bool
					}{
						{"当初予算", b.Total.Initial, nb.Initial, false},
						{"歳出予算現額", b.Total.Current, nb.Current, false},
						{"執行額", b.Total.Executed, nb.Executed, fy > ss[i].FiscalYear-1}, // 古いシートで未確定なら比較しない
					} {
						if d.unless || !d.o.Valid || !d.n.Valid || d.o.Value == d.n.Value {
							continue
						}
						msg := fmt.Sprintf("FY%d %s: 事業年度%d シート %d → 事業年度%d シート %d", fy, d.name, ss[i].FiscalYear, d.o.Value, newest.FiscalYear, d.n.Value)
						row.Revised = append(row.Revised, msg)
						tl.Notes = append(tl.Notes, msg)
					}
				}
			}
		}
		// 要求: FY−1 の行を含む最新シートの翌年度要求額
		for i := len(ss) - 1; i >= 0; i-- {
			if b := ss[i].Budget(fy - 1); b != nil && b.HasTotal && b.Total.NextRequest.Valid {
				row.Request = b.Total.NextRequest
				break
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func trackLoops(ss []*rs.Sheet, lcs []*Lifecycle, th Thresholds) []Loop {
	var loops []Loop
	for i, lc := range lcs {
		S := lc.SheetYear
		lp := Loop{
			SheetYear:        S,
			TeamOpinion:      lc.Evaluation.TeamOpinion,
			Reflection:       lc.Reflection.Status,
			ReflectedGeneral: lc.Reflection.ReflectedGeneral,
			Initial:          lc.Reflection.NextInitial, // Build の NextInitial は FY N+1 = FY S の当初
			NextRequest:      lc.Reflection.Amount,      // FY S+1 の概算要求
		}
		if lc.Reflection.NextInitialState != StateOK {
			lp.Initial = rs.Yen{}
		}
		if lc.Reflection.State != StateOK {
			lp.NextRequest = rs.Yen{}
		}
		// 翌年のシート
		for j := i + 1; j < len(ss); j++ {
			if ss[j].FiscalYear != S+1 {
				continue
			}
			lp.Closed = true
			next := lcs[j] // N = S
			if next.Enacted.State == StateOK {
				lp.Executed, lp.ExecRate = next.Execution.Executed, next.Execution.Rate
				if next.Execution.State != StateOK {
					lp.Executed = rs.Yen{}
				}
			}
			if next.Reflection.NextInitialState == StateOK {
				lp.NextInitial = next.Reflection.NextInitial // FY S+1 当初
			}
			break
		}
		lp.Verdict = judgeLoop(lp)
		lp.Signals = DetectLoop(lp)
		loops = append(loops, lp)
	}
	return loops
}

// judgeLoop は反映状況と翌年当初の増減の整合を判定する。
func judgeLoop(lp Loop) LoopVerdict {
	if !lp.Closed || !lp.NextInitial.Valid || !lp.Initial.Valid {
		return VerdictUnknown
	}
	switch lp.Reflection {
	case "縮減", "廃止", "終了予定":
		if lp.NextInitial.Value > lp.Initial.Value {
			return VerdictContradiction
		}
		return VerdictConsistent
	}
	return VerdictNeutral
}

// SummarizeTimeline は最新シートの Summary に、前年シートの反映状況とループ検証の結果を足す。
func SummarizeTimeline(tl *Timeline, th Thresholds) Summary {
	sm := Summarize(tl.Latest, th)
	sm.SheetYears = tl.SheetYears
	if len(tl.Loops) >= 2 {
		prev := tl.Loops[len(tl.Loops)-2] // 最新の 1 つ前のシートのループ（Closed になりうる）
		sm.PrevReflection = prev.Reflection
		sm.PrevInitial = prev.Initial
		sm.LoopVerdict = prev.Verdict
		sm.Signals |= prev.Signals
	}
	for _, n := range tl.Names {
		if n.Name != tl.Latest.Project.Name {
			sm.Renamed = true
		}
	}
	return sm
}
