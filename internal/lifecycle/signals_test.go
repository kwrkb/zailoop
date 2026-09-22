package lifecycle

import (
	"testing"

	"github.com/kwrkb/zailoop/internal/rs"
)

func TestDetect884(t *testing.T) {
	lc := Build(sheet884(), Options{})
	s := Detect(lc, Thresholds{})
	if !s.Has(SignalCut) {
		t.Errorf("expected SignalCut, got %b", s)
	}
	if s.Has(SignalOutcomeOvershoot) || s.Has(SignalRequestGap) || s.Has(SignalLowExecution) || s.Has(SignalLargeUnused) {
		t.Errorf("unexpected signals %b", s)
	}
	// 閾値を下げれば超過が立つ（達成率 177.3）
	s = Detect(lc, Thresholds{OutcomeOvershoot: 150})
	if !s.Has(SignalOutcomeOvershoot) {
		t.Errorf("expected overshoot at 150, got %b", s)
	}
	// 要求 36,000,000 に対し当初 23,737,000（比 0.66）。閾値 0.7 なら乖離
	s = Detect(lc, Thresholds{RequestGapRatio: 0.7})
	if !s.Has(SignalRequestGap) {
		t.Errorf("expected request gap at 0.7, got %b", s)
	}
	if s.Count() != 2 {
		t.Errorf("Count = %d, want 2 (cut + request gap): %b", s.Count(), s)
	}
}

func TestDetectExceptionStates(t *testing.T) {
	y := func(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }
	zero := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Current: y(0), Executed: y(100)}}}}
	if s := Detect(Build(zero, Options{}), Thresholds{}); !s.Has(SignalExecutionWithoutBudget) || s.Has(SignalLowExecution) {
		t.Errorf("zero current: %b", s)
	}
	neg := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Current: y(1000), Executed: y(600), CarriedOut: y(700)}}}}
	if s := Detect(Build(neg, Options{}), Thresholds{}); !s.Has(SignalNegativeUnused) {
		t.Errorf("negative: %b", s)
	}
	low := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Current: y(10_000_000_000), Executed: y(1_000_000_000), ExecRate: rs.Ratio{Value: 0.1, Valid: true}, CarriedOut: y(0)}}}}
	if s := Detect(Build(low, Options{}), Thresholds{}); !s.Has(SignalLowExecution) || !s.Has(SignalLargeUnused) {
		t.Errorf("low exec + large unused: %b", s)
	}
	small := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Current: y(1000), Executed: y(100), ExecRate: rs.Ratio{Value: 0.1, Valid: true}, CarriedOut: y(0)}}}}
	if s := Detect(Build(small, Options{}), Thresholds{}); s.Has(SignalLargeUnused) {
		t.Errorf("small unused should not be large: %b", s)
	}
	fy := []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100), Current: y(100), Executed: y(50)}}}
	noActual := &rs.Sheet{FiscalYear: 2024, Budgets: fy, Indicators: []rs.Indicator{{Kind: "アウトカム", TargetType: "定量的", Metric: "x"}}}
	if s := Detect(Build(noActual, Options{}), Thresholds{}); !s.Has(SignalNoOutcomeActual) {
		t.Errorf("no actual: %b", s)
	}
	qualitative := &rs.Sheet{FiscalYear: 2024, Budgets: fy, Indicators: []rs.Indicator{{Kind: "アウトカム", TargetType: "定性的", Metric: "x"}}}
	if s := Detect(Build(qualitative, Options{}), Thresholds{}); s.Has(SignalNoOutcomeActual) {
		t.Errorf("qualitative outcome must not be flagged: %b", s)
	}
	newProj := &rs.Sheet{FiscalYear: 2024, Indicators: []rs.Indicator{{Kind: "アウトカム", TargetType: "定量的", Metric: "x"}}}
	if s := Detect(Build(newProj, Options{}), Thresholds{}); s.Has(SignalNoOutcomeActual) {
		t.Errorf("new project must not be flagged: %b", s)
	}
	zz := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Current: y(0), Executed: y(0)}}}}
	if s := Detect(Build(zz, Options{}), Thresholds{}); s.Has(SignalExecutionWithoutBudget) {
		t.Errorf("zero/zero must not be flagged: %b", s)
	}
	short := &rs.Sheet{FiscalYear: 2024, Indicators: []rs.Indicator{{Kind: "アウトカム", Metric: "x", Rates: map[int]string{2023: "45.5%"}}}}
	if s := Detect(Build(short, Options{}), Thresholds{}); !s.Has(SignalOutcomeShortfall) || s.Has(SignalNoOutcomeActual) {
		t.Errorf("shortfall: %b", s)
	}
}

func TestParseRate(t *testing.T) {
	cases := map[string]struct {
		v  float64
		ok bool
	}{"177.3": {177.3, true}, " 96 ": {96, true}, "1,234.5%": {1234.5, true}, "100％": {100, true}, "": {0, false}, "-": {0, false}, "達成": {0, false}}
	for in, want := range cases {
		v, ok := ParseRate(in)
		if ok != want.ok || v != want.v {
			t.Errorf("ParseRate(%q) = %v,%v want %v,%v", in, v, ok, want.v, want.ok)
		}
	}
}

func TestSignalInfoCoversAll(t *testing.T) {
	var all Signal
	for _, d := range SignalInfo {
		if d.Code == "" || d.Label == "" {
			t.Errorf("empty desc for %b", d.Signal)
		}
		all |= d.Signal
	}
	if all != SignalRequestGap|SignalLowExecution|SignalLargeUnused|SignalCut|SignalExecutionWithoutBudget|SignalNegativeUnused|SignalOutcomeShortfall|SignalOutcomeOvershoot|SignalNoOutcomeActual {
		t.Errorf("SignalInfo does not cover all signals: %b", all)
	}
}

func TestSummarize884(t *testing.T) {
	lc := Build(sheet884(), Options{})
	sm := Summarize(lc, Thresholds{})
	if sm.ID != "884" || sm.Ministry != "法務省" || sm.Request.Value != 36000000 || sm.Initial.Value != 23737000 || sm.Current.Value != 40217000 || sm.Executed.Value != 28433000 {
		t.Errorf("amounts: %+v", sm)
	}
	if sm.Unused.Value != 3524000 || sm.UnusedState != StateOK || sm.Reflection != "縮減" || sm.Reflected.Value != -569000 {
		t.Errorf("settlement/reflection: %+v", sm)
	}
	if sm.NextInitial.Value != 11169000 || sm.NextRequest.Value != 24844000 {
		t.Errorf("next: %+v", sm)
	}
	if len(sm.Years) != 4 || sm.Years[0] != 2021 || sm.InitialByYear[3].Value != 11169000 {
		t.Errorf("years: %v %v", sm.Years, sm.InitialByYear)
	}
	if sm.OutcomeCount != 1 || len(sm.OutcomeRates) != 1 || sm.OutcomeRates[0] != 177.3 || !sm.Signals.Has(SignalCut) {
		t.Errorf("outcomes/signals: %+v", sm)
	}
}

func TestSummarizeNewProjectHasNoAmounts(t *testing.T) {
	y := func(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }
	s := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100), NextRequest: y(200)}}}}
	sm := Summarize(Build(s, Options{}), Thresholds{})
	if sm.Request.Valid || sm.Initial.Valid || sm.Current.Valid || sm.Executed.Valid {
		t.Errorf("new project should have no FY N amounts: %+v", sm)
	}
	if sm.NextInitial.Value != 100 || sm.NextRequest.Value != 200 || len(sm.Years) != 1 {
		t.Errorf("next: %+v", sm)
	}
}
