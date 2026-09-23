package lifecycle

import "github.com/kwrkb/zailoop/internal/rs"

// Summary は一覧・横断ビュー用の 1 事業分。JSON タグは持たない（直列化は site の責務）。
type Summary struct {
	ID        string
	SheetYear int // この Summary の元になったシートの事業年度（実績は SheetYear−1）
	Name      string
	Ministry  string
	Category  string

	Request       rs.Yen // ① FY N 概算要求
	Initial       rs.Yen // ② FY N 当初
	Supplementary rs.Yen // ② FY N 補正（当初が 0 で補正だけの事業がある）
	Current       rs.Yen // ② FY N 現額
	Executed      rs.Yen // ③ FY N 執行
	ExecRate      rs.Ratio
	ExecState     State
	CarriedOut    rs.Yen // ④ 翌年度繰越
	Unused        rs.Yen // ④ 不用相当額（UnusedState が OK のとき）
	UnusedDiff    int64  // ④ UnusedState が NeedsReview のときの負の差額
	UnusedState   State

	Reflection  string // ⑥ 反映状況
	Reflected   rs.Yen // ⑥ 反映額（一般会計 + 特別会計）
	NextInitial rs.Yen // FY N+1 当初（参考）
	NextRequest rs.Yen // ⑥ FY N+2 概算要求

	Years         []int    // 予算年度（昇順）
	InitialByYear []rs.Yen // Years と同順の当初予算
	OutcomeCount  int      // アウトカム指標数
	OutcomeRates  []float64
	Signals       Signal

	// 複数年度（SummarizeTimeline）でだけ埋まる
	SheetYears     []int       // 手元にあるシートの事業年度
	PrevReflection string      // 前年シートの反映状況
	PrevInitial    rs.Yen      // 前年シートの FY S 当初（= 当年当初と比較する基準）
	LoopVerdict    LoopVerdict // 前年シートの反映 → 当年当初の整合
	Renamed        bool        // 事業名が年度間で変わった
}

// Summarize は Lifecycle から一覧用サマリを作る。
func Summarize(lc *Lifecycle, th Thresholds) Summary {
	sm := Summary{
		ID: lc.Project.ID, SheetYear: lc.SheetYear, Name: lc.Project.Name, Ministry: lc.Project.Ministry, Category: lc.Project.Category,
		Request: lc.Request.Amount, Initial: lc.Enacted.Initial, Supplementary: lc.Enacted.Supplementary, Current: lc.Enacted.Current,
		Executed: lc.Execution.Executed, ExecRate: lc.Execution.Rate, ExecState: lc.Execution.State,
		CarriedOut: lc.Settlement.CarriedOut, Unused: lc.Settlement.Unused, UnusedDiff: lc.Settlement.Diff, UnusedState: lc.Settlement.UnusedState,
		Reflection: lc.Reflection.Status, NextInitial: lc.Reflection.NextInitial, NextRequest: lc.Reflection.Amount,
	}
	if lc.Request.State != StateOK {
		sm.Request = rs.Yen{}
	}
	if lc.Enacted.State != StateOK {
		sm.Initial, sm.Supplementary, sm.Current = rs.Yen{}, rs.Yen{}, rs.Yen{}
	}
	if lc.Reflection.State != StateOK {
		sm.NextRequest = rs.Yen{}
	}
	if lc.Reflection.NextInitialState != StateOK {
		sm.NextInitial = rs.Yen{}
	}
	if lc.Reflection.ReflectedGeneral.Valid {
		sm.Reflected.Valid = true
		sm.Reflected.Value += lc.Reflection.ReflectedGeneral.Value
	}
	for _, sp := range lc.Reflection.ReflectedSpecial {
		if sp.Amount.Valid {
			sm.Reflected.Valid = true
			sm.Reflected.Value += sp.Amount.Value
		}
	}
	for _, y := range lc.Years {
		sm.Years = append(sm.Years, y.Year)
		sm.InitialByYear = append(sm.InitialByYear, y.Initial)
	}
	sm.OutcomeRates, sm.OutcomeCount = OutcomeRates(lc)
	sm.Signals = Detect(lc, th)
	return sm
}
