package lifecycle

import (
	"strconv"
	"strings"

	"github.com/kwrkb/zailoop/internal/rs"
)

// Thresholds は「ループの断絶」判定の閾値。ゼロ値のフィールドは DefaultThresholds の値で補う。
// 閾値は定数にせず、呼び出し側（CLI 引数）から変えられるようにする。
type Thresholds struct {
	RequestGapRatio  float64 // 当初/要求 がこれ未満で「要求と成立の乖離」
	LowExecRate      float64 // 執行率がこれ未満で「低執行」
	LargeUnusedYen   int64   // 不用相当額がこれ以上、かつ
	LargeUnusedRatio float64 // 不用相当額/現額 がこれ以上で「大きな不用」
	OutcomeShortfall float64 // アウトカム達成率（%）の最小値がこれ未満で「未達」
	OutcomeOvershoot float64 // アウトカム達成率（%）の最大値がこれ超で「超過」
}

// DefaultThresholds は 2024 年度実データの分布から置いた既定値
// （当初/要求比の p10 ≈ 0.55、執行率の p25 ≈ 0.64、不用率の p75 ≈ 0.17）。
func DefaultThresholds() Thresholds {
	return Thresholds{
		RequestGapRatio:  0.5,
		LowExecRate:      0.5,
		LargeUnusedYen:   1_000_000_000,
		LargeUnusedRatio: 0.2,
		OutcomeShortfall: 80,
		OutcomeOvershoot: 200,
	}
}

// WithDefaults はゼロ値のフィールドを既定値で埋めた複製を返す。
func (t Thresholds) WithDefaults() Thresholds {
	d := DefaultThresholds()
	if t.RequestGapRatio == 0 {
		t.RequestGapRatio = d.RequestGapRatio
	}
	if t.LowExecRate == 0 {
		t.LowExecRate = d.LowExecRate
	}
	if t.LargeUnusedYen == 0 {
		t.LargeUnusedYen = d.LargeUnusedYen
	}
	if t.LargeUnusedRatio == 0 {
		t.LargeUnusedRatio = d.LargeUnusedRatio
	}
	if t.OutcomeShortfall == 0 {
		t.OutcomeShortfall = d.OutcomeShortfall
	}
	if t.OutcomeOvershoot == 0 {
		t.OutcomeOvershoot = d.OutcomeOvershoot
	}
	return t
}

// Signal は「ループの断絶」の兆候。ビットフラグで複数持てる。
type Signal uint16

const (
	SignalRequestGap             Signal = 1 << iota // 要求に対して成立した当初予算が小さい
	SignalLowExecution                              // 執行率が低い
	SignalLargeUnused                               // 不用相当額が大きい
	SignalCut                                       // 翌年度反映が「縮減」または「廃止」
	SignalExecutionWithoutBudget                    // 現額が 0 以下なのに執行額がある
	SignalNegativeUnused                            // 現額−執行−繰越 が負（要確認）
	SignalOutcomeShortfall                          // アウトカム達成率が低い
	SignalOutcomeOvershoot                          // アウトカム達成率が高すぎる（目標設定が甘い可能性）
	SignalNoOutcomeActual                           // アウトカム指標があるのに確定年度の達成率がない
	SignalReflectionContradicted                    // 縮減・廃止・終了予定なのに翌年の当初予算が増えた（複数年度）
	SignalRequestZeroed                             // 概算要求があったのに翌年の当初予算が 0（複数年度）
)

// Has は s が f をすべて含むとき true。
func (s Signal) Has(f Signal) bool { return s&f == f }

// Count は立っているビット数。
func (s Signal) Count() int {
	n := 0
	for ; s != 0; s >>= 1 {
		n += int(s & 1)
	}
	return n
}

// SignalDesc は Signal 1 つ分の説明。JSON の meta とフロントのラベルの唯一の源。
type SignalDesc struct {
	Signal      Signal
	Code        string
	Label       string
	Description string
}

// SignalInfo は定義順の Signal 一覧。
var SignalInfo = []SignalDesc{
	{SignalRequestGap, "request_gap", "要求と成立の乖離", "当初予算が概算要求額に対して小さい"},
	{SignalLowExecution, "low_execution", "低執行", "執行率が低い"},
	{SignalLargeUnused, "large_unused", "大きな不用", "不用相当額が額・率ともに大きい"},
	{SignalCut, "cut", "縮減・廃止", "翌年度の概算要求への反映状況が縮減または廃止"},
	{SignalExecutionWithoutBudget, "execution_without_budget", "予算なし執行", "歳出予算現額が 0 以下なのに執行額がある"},
	{SignalNegativeUnused, "negative_unused", "差額が負", "現額−執行−翌年度繰越 が負で整合しない"},
	{SignalOutcomeShortfall, "outcome_shortfall", "成果未達", "アウトカム達成率の最小値が低い"},
	{SignalOutcomeOvershoot, "outcome_overshoot", "成果超過", "アウトカム達成率の最大値が高すぎる"},
	{SignalNoOutcomeActual, "no_outcome_actual", "成果実績なし", "定量的アウトカム指標があるのに確定年度の達成率がない（新規事業を除く）"},
	{SignalReflectionContradicted, "reflection_contradicted", "反映要確認", "前年シートの反映状況が縮減・廃止・終了予定だったのに、翌年の当初予算が増えた（事業の再編・移管などの可能性もある）"},
	{SignalRequestZeroed, "request_zeroed", "要求ゼロ査定", "前年シートで概算要求があったのに、翌年の当初予算が 0"},
}

// DetectLoop は翌年のシートと突き合わせたループから兆候を判定する。
func DetectLoop(lp Loop) Signal {
	var s Signal
	if lp.Verdict == VerdictContradiction {
		s |= SignalReflectionContradicted
	}
	if lp.Closed && lp.NextRequest.Valid && lp.NextRequest.Value > 0 && lp.NextInitial.Valid && lp.NextInitial.Value == 0 {
		s |= SignalRequestZeroed
	}
	return s
}

// Detect は Lifecycle から断絶の兆候を判定する。判定は Lifecycle のフィールドだけから求める。
func Detect(lc *Lifecycle, th Thresholds) Signal {
	th = th.WithDefaults()
	var s Signal
	if lc.Request.State == StateOK && lc.Enacted.State == StateOK && lc.Request.Amount.Value > 0 && lc.Enacted.Initial.Valid {
		if float64(lc.Enacted.Initial.Value)/float64(lc.Request.Amount.Value) < th.RequestGapRatio {
			s |= SignalRequestGap
		}
	}
	if lc.Execution.State == StateOK && lc.Execution.Rate.Valid && lc.Execution.Rate.Value < th.LowExecRate {
		s |= SignalLowExecution
	}
	if lc.Settlement.UnusedState == StateOK && lc.Enacted.Current.Valid && lc.Enacted.Current.Value > 0 {
		u := lc.Settlement.Unused.Value
		if u >= th.LargeUnusedYen && float64(u)/float64(lc.Enacted.Current.Value) >= th.LargeUnusedRatio {
			s |= SignalLargeUnused
		}
	}
	if lc.Reflection.Status == "縮減" || lc.Reflection.Status == "廃止" {
		s |= SignalCut
	}
	if lc.Execution.State == StateNotComputable {
		s |= SignalExecutionWithoutBudget
	}
	if lc.Settlement.UnusedState == StateNeedsReview {
		s |= SignalNegativeUnused
	}
	mn, mx, ok, outcomes := outcomeRange(lc)
	if ok {
		if mn < th.OutcomeShortfall {
			s |= SignalOutcomeShortfall
		}
		if mx > th.OutcomeOvershoot {
			s |= SignalOutcomeOvershoot
		}
	} else if outcomes > 0 && lc.Enacted.State == StateOK {
		// 定量的アウトカムがあり、FY N の予算行もあるのに達成率がない（新規事業は除く）
		s |= SignalNoOutcomeActual
	}
	return s
}

// OutcomeRates は確定年度のアウトカム達成率（%）のうち数値として読めたものと、
// 定量的アウトカム指標の数を返す。定性的な指標は達成率を持たないので数えない。
func OutcomeRates(lc *Lifecycle) (rates []float64, outcomes int) {
	for _, o := range lc.Evaluation.Outcomes {
		if o.Kind != "アウトカム" || o.TargetType == "定性的" {
			continue
		}
		outcomes++
		if v, ok := ParseRate(o.Rate); ok {
			rates = append(rates, v)
		}
	}
	return rates, outcomes
}

// outcomeRange は確定年度のアウトカム達成率の最小・最大。ok は達成率が 1 つ以上読めたとき true。
func outcomeRange(lc *Lifecycle) (mn, mx float64, ok bool, outcomes int) {
	rates, outcomes := OutcomeRates(lc)
	if len(rates) == 0 {
		return 0, 0, false, outcomes
	}
	mn, mx = rates[0], rates[0]
	for _, r := range rates[1:] {
		mn = min(mn, r)
		mx = max(mx, r)
	}
	return mn, mx, true, outcomes
}

// ParseRate は "177.3" や "177.3%"、"1,234.5" を % の数値として読む。読めなければ false。
func ParseRate(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	s = strings.TrimSuffix(s, "％")
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// Evidence は立っている兆候 1 つの根拠（判定に使った観測値と閾値）。文言にするのは site の責務。
// 使うフィールドは兆候ごとに決まる（Explain のコメント参照）。
type Evidence struct {
	Signal          Signal
	Observed        float64 // 比率（0〜1）または達成率（%）
	Threshold       float64 // Observed と比べた閾値
	Amount          rs.Yen  // 観測した金額
	ThresholdAmount int64   // Amount と比べた閾値（大きな不用）
	Base            rs.Yen  // 比較元の金額
	Text            string  // 反映状況など文字列の根拠
	Year            int     // ループの兆候で比べたシートの事業年度 S（FY S 当初 → FY S+1）
}

// Explain は s に立っている兆候について、Detect / DetectLoop が見た値を返す。条件は再判定しない。
// 兆候ごとの中身:
//   - request_gap: Observed=当初/要求, Threshold, Amount=当初, Base=要求
//   - low_execution: Observed=執行率, Threshold
//   - large_unused: Amount=不用相当額, ThresholdAmount, Observed=不用/現額, Threshold
//   - cut: Text=反映状況
//   - execution_without_budget: Amount=執行額, Base=現額
//   - negative_unused: Amount=差額（負）
//   - outcome_shortfall / outcome_overshoot: Observed=達成率の最小 / 最大（%）, Threshold
//   - no_outcome_actual: なし
//   - reflection_contradicted: Year=S, Text=S シートの反映状況, Base=FY S 当初, Amount=FY S+1 当初
//   - request_zeroed: Year=S, Base=S シートの FY S+1 概算要求, Amount=FY S+1 当初
func Explain(tl *Timeline, s Signal, th Thresholds) []Evidence {
	th = th.WithDefaults()
	lc := tl.Latest
	var prev Loop
	if len(tl.Loops) >= 2 {
		prev = tl.Loops[len(tl.Loops)-2] // SummarizeTimeline と同じループ
	}
	var out []Evidence
	for _, d := range SignalInfo {
		if !s.Has(d.Signal) {
			continue
		}
		ev := Evidence{Signal: d.Signal}
		switch d.Signal {
		case SignalRequestGap:
			ev.Amount, ev.Base, ev.Threshold = lc.Enacted.Initial, lc.Request.Amount, th.RequestGapRatio
			if lc.Request.Amount.Value > 0 {
				ev.Observed = float64(lc.Enacted.Initial.Value) / float64(lc.Request.Amount.Value)
			}
		case SignalLowExecution:
			ev.Observed, ev.Threshold = lc.Execution.Rate.Value, th.LowExecRate
		case SignalLargeUnused:
			ev.Amount, ev.ThresholdAmount, ev.Threshold = lc.Settlement.Unused, th.LargeUnusedYen, th.LargeUnusedRatio
			if lc.Enacted.Current.Value > 0 {
				ev.Observed = float64(lc.Settlement.Unused.Value) / float64(lc.Enacted.Current.Value)
			}
		case SignalCut:
			ev.Text = lc.Reflection.Status
		case SignalExecutionWithoutBudget:
			ev.Amount, ev.Base = lc.Execution.Executed, lc.Enacted.Current
		case SignalNegativeUnused:
			ev.Amount = rs.Yen{Value: lc.Settlement.Diff, Valid: true}
		case SignalOutcomeShortfall:
			ev.Observed, _, _, _ = outcomeRange(lc)
			ev.Threshold = th.OutcomeShortfall
		case SignalOutcomeOvershoot:
			_, ev.Observed, _, _ = outcomeRange(lc)
			ev.Threshold = th.OutcomeOvershoot
		case SignalReflectionContradicted:
			ev.Year, ev.Text, ev.Base, ev.Amount = prev.SheetYear, prev.Reflection, prev.Initial, prev.NextInitial
		case SignalRequestZeroed:
			ev.Year, ev.Base, ev.Amount = prev.SheetYear, prev.NextRequest, prev.NextInitial
		}
		out = append(out, ev)
	}
	return out
}
