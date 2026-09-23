// Package lifecycle は rs.Sheet から予算ライフサイクル 6 段階
// 「要求 → 成立 → 執行 → 決算 → 評価 → 翌年度反映」を組み立てる。
// I/O を持たず、列名も知らない。欠測・算出不能は数値とは別に State で表す。
//
// 年度の考え方（docs/data-survey.md §4）:
//   - 事業年度 S のシートには予算年度 S-3 〜 S の予算があり、執行実績は S-1 までしかない。
//   - 「執行が確定した最新の予算年度」を N = S-1 とし、FY N を軸に 6 段階を並べる。
//   - ① 要求: 予算年度 N-1 の行の翌年度要求額（= FY N の概算要求）
//   - ② 成立: 予算年度 N の当初・補正・繰越・予備費・現額
//   - ③ 執行: 予算年度 N の執行額・執行率
//   - ④ 決算: 予算年度 N の翌年度繰越と、不用相当額 = 現額 − 執行 − 翌年度繰越（導出）
//   - ⑤ 評価: 4-1 の点検・所見と、3-1 の FY N 実績
//   - ⑥ 翌年度反映: 4-1 の反映状況・反映額と、予算年度 N+1 の行の翌年度要求額（= FY N+2 の概算要求）
package lifecycle

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kwrkb/zailoop/internal/rs"
)

// State は各段階の値の表示状態。
type State int

const (
	StateOK            State = iota // 値がある
	StateMissing                    // その年度の行がない（新規事業など）
	StateNotComputable              // 前提が欠けて算出対象外（現額 0 以下など）
	StateNeedsReview                // 値はあるが整合しない（差額が負など）
)

func (s State) String() string {
	switch s {
	case StateOK:
		return "ok"
	case StateMissing:
		return "データなし"
	case StateNotComputable:
		return "算出対象外"
	case StateNeedsReview:
		return "要確認"
	}
	return fmt.Sprintf("State(%d)", int(s))
}

// Request は ① 要求。
type Request struct {
	FiscalYear int    // 要求対象年度（= N）
	Amount     rs.Yen // 予算年度 N-1 の翌年度要求額
	State      State
}

// Enacted は ② 成立。
type Enacted struct {
	FiscalYear    int
	Initial       rs.Yen
	Supplementary rs.Yen
	CarriedIn     rs.Yen
	Reserve       rs.Yen
	Current       rs.Yen // 歳出予算現額
	State         State
	Items         []ItemLine // 2-2 を予算種別で集約（出現順）
	Notes         string     // その他特記事項（内数表記の説明などが入る）
}

// ItemLine は 2-2 の予算種別ごとの集約。
type ItemLine struct {
	Kind   string
	Amount rs.Yen
	Count  int
}

// Execution は ③ 執行。
type Execution struct {
	FiscalYear int
	Executed   rs.Yen
	Rate       rs.Ratio
	State      State
}

// Settlement は ④ 決算。Unused は導出値。
type Settlement struct {
	FiscalYear  int
	CarriedOut  rs.Yen
	Unused      rs.Yen // 現額 − 執行 − 翌年度繰越（State が OK のときだけ意味を持つ）
	UnusedState State
	Diff        int64 // State が NeedsReview のときの差額（負値）
}

// OutcomeLine は ⑤ の成果指標 1 件（FY N の目標と実績）。
type OutcomeLine struct {
	Kind       string // アウトプット/アウトカム
	TargetType string // 定量的/定性的
	Term       string
	Goal       string
	Metric     string
	Unit       string
	Target     string // FY N の目標値（空あり）
	Actual     string // FY N の実績値
	Rate       string // FY N の達成率
}

// Evaluation は ⑤ 評価。ダッシュだけの記述は空に正規化する。
type Evaluation struct {
	SelfCheck         string
	Improvement       string
	ExternalYear      string
	ExternalTarget    string
	ExternalOpinion   string
	TeamOpinion       string
	TeamOpinionDetail string
	Outcomes          []OutcomeLine
}

// Reflection は ⑥ 翌年度反映。
type Reflection struct {
	Status           string // 反映状況（現状通り/縮減/廃止 …）
	Detail           string
	ReflectedGeneral rs.Yen
	ReflectedSpecial []rs.SpecialReflection
	RequestYear      int    // 次の要求対象年度（= N+2）
	Amount           rs.Yen // 予算年度 N+1 の翌年度要求額
	Demand           rs.Yen // 要望額（会計別行の和）
	ChangeReason     string
	State            State
	NextInitial      rs.Yen // 参考: FY N+1 の当初予算
	NextInitialState State
}

// YearLine は予算年度ごとの主要 3 値。詳細ページの年度推移と一覧のスパークライン用。
type YearLine struct {
	Year     int
	Initial  rs.Yen
	Current  rs.Yen
	Executed rs.Yen
	HasTotal bool
}

// PayeeSummary は 5-1 のブロック概要。
type PayeeSummary struct {
	Number     string
	Name       string
	Role       string
	PayeeCount string
	Total      rs.Yen
}

// Lifecycle は 1 事業の 6 段階。
type Lifecycle struct {
	Project    rs.Project
	SheetYear  int // 事業年度 S
	ActualYear int // 執行が確定した最新の予算年度 N

	Request    Request
	Enacted    Enacted
	Execution  Execution
	Settlement Settlement
	Evaluation Evaluation
	Reflection Reflection

	Payees      []PayeeSummary // 金額上位。BlockCount と合わせて「ほか n ブロック」を出す
	BlockCount  int
	BudgetYears []int      // シートにある予算年度（昇順）
	Years       []YearLine // 予算年度ごとの当初・現額・執行（昇順）
	Notes       []string   // 整合チェックの警告（rs の Issues を含む）

	Related []rs.Related // 1-5 の関連事業（シートの記載どおり）
	// AllZero は最新シートの全予算年度で当初・補正・現額・執行・翌年度要求がすべて 0（または空欄）。
	// 予算が親事業などにまとめて計上され、事業単位の額が出ていない事業に多い。
	AllZero bool
	// ZeroNotes は AllZero のときだけ、全予算年度の「その他特記事項」「主な増減理由」を重複を除いて並べたもの。
	// 0 の理由（内数表記、国庫債務負担行為など）が FY N 以外の行にだけ書かれている事業がある。
	ZeroNotes []string
}

// Parents は関連事業のうち関連性が「親事業」のもの。
func (lc *Lifecycle) Parents() []rs.Related {
	var ps []rs.Related
	for _, r := range lc.Related {
		if r.Kind == "親事業" {
			ps = append(ps, r)
		}
	}
	return ps
}

// Options は Build の設定。
type Options struct {
	ActualYear int // 0 なら SheetYear-1
	TopBlocks  int // 0 なら 3
}

// Build は Sheet から Lifecycle を組み立てる。
func Build(s *rs.Sheet, opt Options) *Lifecycle {
	n := opt.ActualYear
	if n == 0 {
		n = s.FiscalYear - 1
	}
	top := opt.TopBlocks
	if top == 0 {
		top = 3
	}
	lc := &Lifecycle{Project: s.Project, SheetYear: s.FiscalYear, ActualYear: n}
	for _, b := range s.Budgets {
		lc.BudgetYears = append(lc.BudgetYears, b.Year)
		lc.Years = append(lc.Years, YearLine{Year: b.Year, Initial: b.Total.Initial, Current: b.Total.Current, Executed: b.Total.Executed, HasTotal: b.HasTotal})
		for _, is := range b.Issues {
			lc.Notes = append(lc.Notes, fmt.Sprintf("予算年度%d: %s", b.Year, is))
		}
		if b.Year > n && b.Total.Executed.Valid && b.Total.Executed.Value != 0 {
			lc.Notes = append(lc.Notes, fmt.Sprintf("予算年度%d に執行額があります（確定年度 %d より後）", b.Year, n))
		}
	}
	lc.Request = buildRequest(s, n)
	lc.Enacted = buildEnacted(s, n)
	lc.Execution = buildExecution(s, n)
	lc.Settlement = buildSettlement(s, n)
	lc.Evaluation = buildEvaluation(s, n)
	lc.Reflection = buildReflection(s, n)
	lc.Payees, lc.BlockCount = buildPayees(s, top)
	lc.Related = s.Related
	lc.AllZero = allZero(s.Budgets)
	if lc.AllZero {
		lc.ZeroNotes = zeroNotes(s.Budgets)
	}
	return lc
}

func zeroNotes(budgets []rs.BudgetYear) []string {
	var out []string
	seen := map[string]bool{}
	for _, b := range budgets {
		for _, v := range [][2]string{{"その他特記事項", b.Total.Notes}, {"主な増減理由", b.Total.ChangeReason}} {
			text := Clean(v[1])
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true
			out = append(out, fmt.Sprintf("%s（予算年度%d）: %s", v[0], b.Year, text))
		}
	}
	return out
}

// allZero は合計行が 1 つ以上あり、その金額がすべて 0 か空欄かを返す。
func allZero(budgets []rs.BudgetYear) bool {
	seen := false
	for _, b := range budgets {
		if !b.HasTotal {
			continue
		}
		seen = true
		t := b.Total
		for _, v := range []rs.Yen{t.Initial, t.Supplementary, t.Current, t.Executed, t.NextRequest} {
			if v.Valid && v.Value != 0 {
				return false
			}
		}
	}
	return seen
}

func buildRequest(s *rs.Sheet, n int) Request {
	r := Request{FiscalYear: n, State: StateMissing}
	if b := s.Budget(n - 1); b != nil && b.HasTotal {
		r.Amount = b.Total.NextRequest
		if r.Amount.Valid {
			r.State = StateOK
		}
	}
	return r
}

func buildEnacted(s *rs.Sheet, n int) Enacted {
	e := Enacted{FiscalYear: n, State: StateMissing}
	if b := s.Budget(n); b != nil && b.HasTotal {
		t := b.Total
		e.Initial, e.Supplementary, e.CarriedIn, e.Reserve, e.Current = t.Initial, t.Supplementary, t.CarriedIn, t.Reserve, t.Current
		e.Notes = Clean(t.Notes)
		e.State = StateOK
	}
	var order []string
	agg := map[string]*ItemLine{}
	for _, it := range s.Items {
		if it.Year != n {
			continue
		}
		l, ok := agg[it.Kind]
		if !ok {
			l = &ItemLine{Kind: it.Kind}
			agg[it.Kind] = l
			order = append(order, it.Kind)
		}
		l.Count++
		if it.Amount.Valid {
			l.Amount.Valid = true
			l.Amount.Value += it.Amount.Value
		}
	}
	for _, k := range order {
		e.Items = append(e.Items, *agg[k])
	}
	return e
}

func buildExecution(s *rs.Sheet, n int) Execution {
	x := Execution{FiscalYear: n, State: StateMissing}
	b := s.Budget(n)
	if b == nil || !b.HasTotal {
		return x
	}
	x.Executed = b.Total.Executed
	x.Rate = b.Total.ExecRate
	switch {
	case !x.Executed.Valid:
		x.State = StateMissing
	case (!b.Total.Current.Valid || b.Total.Current.Value <= 0) && x.Executed.Value != 0:
		// 現額が 0 以下なのに執行額がある（予算が別事業に計上されているケース）。執行額は原値を出し、率は対象外。
		x.State = StateNotComputable
		x.Rate = rs.Ratio{}
	case !b.Total.Current.Valid || b.Total.Current.Value <= 0:
		// 現額 0・執行 0。値はあるが率は定義できない。
		x.State = StateOK
		x.Rate = rs.Ratio{}
	default:
		x.State = StateOK
	}
	return x
}

func buildSettlement(s *rs.Sheet, n int) Settlement {
	st := Settlement{FiscalYear: n, UnusedState: StateMissing}
	b := s.Budget(n)
	if b == nil || !b.HasTotal {
		return st
	}
	t := b.Total
	st.CarriedOut = t.CarriedOut
	if !t.Current.Valid || !t.Executed.Valid {
		return st
	}
	if t.Current.Value <= 0 {
		st.UnusedState = StateNotComputable
		return st
	}
	diff := t.Current.Value - t.Executed.Value
	if t.CarriedOut.Valid {
		diff -= t.CarriedOut.Value
	}
	if diff < 0 {
		st.UnusedState = StateNeedsReview
		st.Diff = diff
		return st
	}
	st.Unused = rs.Yen{Value: diff, Valid: true}
	st.UnusedState = StateOK
	return st
}

func buildEvaluation(s *rs.Sheet, n int) Evaluation {
	ev := s.Evaluation
	e := Evaluation{
		SelfCheck:         Clean(ev.SelfCheck),
		Improvement:       Clean(ev.Improvement),
		ExternalYear:      Clean(ev.ExternalYear),
		ExternalTarget:    Clean(ev.ExternalTarget),
		ExternalOpinion:   Clean(ev.ExternalOpinion),
		TeamOpinion:       Clean(ev.TeamOpinion),
		TeamOpinionDetail: Clean(ev.TeamOpinionDetail),
	}
	for _, in := range s.Indicators {
		if in.Kind == "アクティビティ" || in.Kind == "" {
			continue
		}
		e.Outcomes = append(e.Outcomes, OutcomeLine{
			Kind: in.Kind, TargetType: in.TargetType, Term: in.Term, Goal: in.Goal, Metric: in.Metric, Unit: in.Unit,
			Target: in.Targets[n], Actual: in.Actuals[n], Rate: in.Rates[n],
		})
	}
	return e
}

func buildReflection(s *rs.Sheet, n int) Reflection {
	ev := s.Evaluation
	r := Reflection{
		Status:           Clean(ev.Reflection),
		Detail:           Clean(ev.ReflectionDetail),
		ReflectedGeneral: ev.ReflectedGeneral,
		ReflectedSpecial: ev.ReflectedSpecial,
		RequestYear:      n + 2,
		State:            StateMissing,
		NextInitialState: StateMissing,
	}
	if b := s.Budget(n + 1); b != nil && b.HasTotal {
		r.Amount = b.Total.NextRequest
		r.ChangeReason = Clean(b.Total.ChangeReason)
		if r.Amount.Valid {
			r.State = StateOK
		}
		for _, a := range b.Accounts {
			if a.Demand.Valid {
				r.Demand.Valid = true
				r.Demand.Value += a.Demand.Value
			}
		}
		r.NextInitial = b.Total.Initial
		if r.NextInitial.Valid {
			r.NextInitialState = StateOK
		}
	}
	return r
}

func buildPayees(s *rs.Sheet, top int) ([]PayeeSummary, int) {
	var all []PayeeSummary
	for _, b := range s.Blocks {
		all = append(all, PayeeSummary{Number: b.Number, Name: b.Name, Role: b.Role, PayeeCount: b.PayeeCount, Total: b.Total})
	}
	sort.SliceStable(all, func(i, j int) bool {
		vi, vj := int64(-1), int64(-1)
		if all[i].Total.Valid {
			vi = all[i].Total.Value
		}
		if all[j].Total.Valid {
			vj = all[j].Total.Value
		}
		return vi > vj
	})
	if len(all) > top {
		return all[:top], len(s.Blocks)
	}
	return all, len(s.Blocks)
}

// Clean は「該当なし」を意味するダッシュだけの記述を空文字にし、前後の空白を落とす。
// 4-1 には -, ー, －, --, ｰ, ―, ‐ などが混在する（docs/data-survey.md §6）。
func Clean(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, r := range s {
		switch r {
		case '-', 'ー', '－', 'ｰ', '―', '‐', '−', '—', '–', ' ', '　':
		default:
			return s
		}
	}
	return ""
}
