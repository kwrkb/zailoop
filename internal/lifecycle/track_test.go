package lifecycle

import (
	"strings"
	"testing"

	"github.com/kwrkb/zailoop/internal/rs"
)

// sheet884y2025 は 884 の 2025 年度シートを模したもの（FY2025 当初 = 縮減後、FY2024 執行あり、FY2024 当初を改訂）。
func sheet884y2025() *rs.Sheet {
	return &rs.Sheet{
		FiscalYear: 2025,
		Project:    rs.Project{ID: "884", Name: "法教育の推進（改）", Ministry: "法務省"},
		Budgets: []rs.BudgetYear{
			{Year: 2022, HasTotal: true, Total: rs.BudgetTotal{Initial: y(30261000), Current: y(38481000), Executed: y(26000000), NextRequest: y(36000000)}},
			{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(23737000), Current: y(40217000), Executed: y(28433000), CarriedOut: y(8260000), NextRequest: y(42023000)}},
			{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(11200000), Current: y(19429000), Executed: y(15000000), ExecRate: rs.Ratio{Value: 0.772, Valid: true}, CarriedOut: y(0), NextRequest: y(24844000)}},
			{Year: 2025, HasTotal: true, Total: rs.BudgetTotal{Initial: y(9000000), Current: y(9000000), Executed: y(0), NextRequest: y(12000000)}},
		},
		Evaluation: rs.Evaluation{TeamOpinion: "現状通り", Reflection: "現状通り"},
	}
}

func TestTrack884(t *testing.T) {
	tl := Track([]*rs.Sheet{sheet884y2025(), sheet884()}, Thresholds{}) // 逆順で渡す
	if tl.ID != "884" || len(tl.SheetYears) != 2 || tl.SheetYears[0] != 2024 || tl.Latest.SheetYear != 2025 {
		t.Fatalf("timeline = %+v", tl)
	}
	if len(tl.Names) != 2 || tl.Names[1].Name != "法教育の推進（改）" {
		t.Errorf("names = %+v", tl.Names)
	}
	// 予算年度 2021〜2025
	if len(tl.Years) != 5 || tl.Years[0].FY != 2021 || tl.Years[4].FY != 2025 {
		t.Fatalf("years = %+v", tl.Years)
	}
	y2024 := tl.Years[3]
	if y2024.Source != 2025 || y2024.Initial.Value != 11200000 || y2024.Executed.Value != 15000000 || y2024.ExecState != StateOK || y2024.Request.Value != 42023000 {
		t.Errorf("FY2024 row = %+v", y2024)
	}
	if len(y2024.Revised) != 1 || !strings.Contains(y2024.Revised[0].String(), "当初予算") || !strings.Contains(y2024.Revised[0].String(), "11169000") {
		t.Errorf("revised = %v", y2024.Revised)
	}
	y2025 := tl.Years[4]
	if y2025.Source != 2025 || y2025.Initial.Value != 9000000 || y2025.Request.Value != 24844000 || y2025.ExecState != StateMissing || y2025.Executed.Valid {
		t.Errorf("FY2025 row = %+v", y2025)
	}
	y2021 := tl.Years[0]
	if y2021.Source != 2024 || y2021.Initial.Value != 28854000 {
		t.Errorf("FY2021 row = %+v", y2021)
	}
	// ループ: 2024 シートの縮減 → FY2025 当初 9,000,000 < FY2024 当初 11,169,000 → 整合
	if len(tl.Loops) != 2 {
		t.Fatalf("loops = %+v", tl.Loops)
	}
	l24 := tl.Loops[0]
	if !l24.Closed || l24.Reflection != "縮減" || l24.Initial.Value != 11169000 || l24.NextRequest.Value != 24844000 || l24.NextInitial.Value != 9000000 || l24.Executed.Value != 15000000 || l24.Verdict != VerdictConsistent {
		t.Errorf("loop 2024 = %+v", l24)
	}
	l25 := tl.Loops[1]
	if l25.Closed || l25.Verdict != VerdictUnknown || l25.Reflection != "現状通り" {
		t.Errorf("loop 2025 = %+v", l25)
	}
	sm := SummarizeTimeline(tl, Thresholds{})
	if sm.ID != "884" || sm.PrevReflection != "縮減" || sm.LoopVerdict != VerdictConsistent || !sm.Renamed || len(sm.SheetYears) != 2 || sm.Initial.Value != 11200000 {
		t.Errorf("summary = %+v", sm)
	}
	if sm.Signals.Has(SignalReflectionContradicted) {
		t.Errorf("unexpected contradiction")
	}
	// FY2024 当初 11,169,000 → 11,200,000 は 0.28% で、既定の閾値 1% 未満
	if sm.Signals.Has(SignalAmountRevised) {
		t.Errorf("unexpected amount_revised: %v", QualifyingRevisions(tl, Thresholds{}))
	}
	if !SummarizeTimeline(tl, Thresholds{RevisionRatio: 0.001}).Signals.Has(SignalAmountRevised) {
		t.Errorf("amount_revised not detected with 0.1%%")
	}
}

func TestTrackContradictionAndZeroed(t *testing.T) {
	s24 := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100), NextRequest: y(150)}},
	}, Evaluation: rs.Evaluation{Reflection: "廃止"}}
	s25 := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100), Current: y(100), Executed: y(80)}},
		{Year: 2025, HasTotal: true, Total: rs.BudgetTotal{Initial: y(200)}},
	}}
	tl := Track([]*rs.Sheet{s24, s25}, Thresholds{})
	if tl.Loops[0].Verdict != VerdictContradiction || !tl.Loops[0].Signals.Has(SignalReflectionContradicted) {
		t.Errorf("loop = %+v", tl.Loops[0])
	}
	if evs := Explain(tl, SignalReflectionContradicted, Thresholds{}); len(evs) != 1 || evs[0].Year != 2024 || evs[0].Text != "廃止" || evs[0].Base.Value != 100 || evs[0].Amount.Value != 200 {
		t.Errorf("contradiction evidence = %+v", evs)
	}
	s25.Budgets[1].Total.Initial = y(0)
	tl = Track([]*rs.Sheet{s24, s25}, Thresholds{})
	if tl.Loops[0].Verdict != VerdictConsistent || !tl.Loops[0].Signals.Has(SignalRequestZeroed) {
		t.Errorf("zeroed loop = %+v", tl.Loops[0])
	}
	if evs := Explain(tl, SignalRequestZeroed, Thresholds{}); len(evs) != 1 || evs[0].Base.Value != 150 || evs[0].Amount.Value != 0 || !evs[0].Amount.Valid {
		t.Errorf("zeroed evidence = %+v", evs)
	}
	// 現状通りは判定対象外
	s24.Evaluation.Reflection = "現状通り"
	if tl := Track([]*rs.Sheet{s24, s25}, Thresholds{}); tl.Loops[0].Verdict != VerdictNeutral {
		t.Errorf("neutral = %+v", tl.Loops[0])
	}
}

func TestAmountRevised(t *testing.T) {
	// 古いシート 2024 の FY2023（確定）と FY2024（当年度）、新しいシート 2025 で値を変えて比べる
	old := func() *rs.Sheet {
		return &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
			{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(1000), Current: y(1000), Executed: y(900)}},
			{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(1000), Current: y(1000)}},
		}}
	}
	cases := []struct {
		name string
		set  func(b []rs.BudgetYear) // 新しいシートの FY2023, FY2024
		want []string                // QualifyingRevisions の Item（差の大きい順）
	}{
		{"変化なし", func(b []rs.BudgetYear) {}, nil},
		{"当年度の当初が 1% 以上", func(b []rs.BudgetYear) { b[1].Total.Initial = y(1010) }, []string{"当初予算"}},
		{"当年度の当初が 1% 未満", func(b []rs.BudgetYear) { b[1].Total.Initial = y(1009) }, nil},
		{"当年度の現額が変わるだけ", func(b []rs.BudgetYear) { b[1].Total.Current = y(3000) }, nil},
		{"当年度の現額が 0 になる", func(b []rs.BudgetYear) { b[1].Total.Current = y(0) }, []string{"歳出予算現額"}},
		{"確定年度の現額", func(b []rs.BudgetYear) { b[0].Total.Current = y(1100) }, []string{"歳出予算現額"}},
		{"確定年度の執行と当初（差の大きい順）", func(b []rs.BudgetYear) { b[0].Total.Executed = y(500); b[0].Total.Initial = y(1200) }, []string{"執行額", "当初予算"}},
	}
	for _, c := range cases {
		s25 := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
			{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(1000), Current: y(1000), Executed: y(900)}},
			{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(1000), Current: y(1000), Executed: y(800)}},
		}}
		c.set(s25.Budgets)
		tl := Track([]*rs.Sheet{old(), s25}, Thresholds{})
		var got []string
		for _, r := range QualifyingRevisions(tl, Thresholds{}) {
			got = append(got, r.Item)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
		if has := SummarizeTimeline(tl, Thresholds{}).Signals.Has(SignalAmountRevised); has != (len(c.want) > 0) {
			t.Errorf("%s: signal = %v", c.name, has)
		}
	}
	// 旧が 0 なら閾値によらず数える。根拠は差の大きい順の改訂
	s24 := old()
	s24.Budgets[0].Total.Initial = y(0)
	s25 := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
		{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(5), Current: y(1000), Executed: y(900)}},
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(0), Current: y(0)}},
	}}
	tl := Track([]*rs.Sheet{s24, s25}, Thresholds{RevisionRatio: 0.5})
	evs := Explain(tl, SignalAmountRevised, Thresholds{RevisionRatio: 0.5})
	if len(evs) != 1 || len(evs[0].Revisions) != 3 {
		t.Fatalf("evidence = %+v", evs)
	}
	if r := evs[0].Revisions[0]; r.FY != 2024 || r.Old.Value != 1000 || r.New.Value != 0 || r.OldSheet != 2024 || r.NewSheet != 2025 {
		t.Errorf("largest = %+v", r)
	}
	if r := evs[0].Revisions[2]; r.FY != 2023 || r.Item != "当初予算" || r.Old.Value != 0 || r.New.Value != 5 {
		t.Errorf("from zero = %+v", r)
	}
}

func TestTrackSingleSheet(t *testing.T) {
	tl := Track([]*rs.Sheet{sheet884()}, Thresholds{})
	if len(tl.Loops) != 1 || tl.Loops[0].Closed || len(tl.Years) != 4 {
		t.Errorf("single = %+v", tl)
	}
	sm := SummarizeTimeline(tl, Thresholds{})
	if sm.PrevReflection != "" || sm.LoopVerdict != VerdictUnknown {
		t.Errorf("summary = %+v", sm)
	}
	if Track(nil, Thresholds{}) != nil {
		t.Error("nil expected")
	}
}

func TestTrackPastParents(t *testing.T) {
	parent := func(id string) rs.Related { return rs.Related{ID: id, Name: "親" + id, Kind: "親事業"} }
	old := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "5"}, Related: []rs.Related{parent("497"), parent("1"), {ID: "7", Kind: "その他関連先"}}}
	cur := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "5"}, Related: []rs.Related{parent("1")}}
	tl := Track([]*rs.Sheet{cur, old}, Thresholds{})
	if len(tl.PastParents) != 1 || tl.PastParents[0].SheetYear != 2024 || tl.PastParents[0].ID != "497" {
		t.Errorf("PastParents = %+v", tl.PastParents)
	}
	if tl := Track([]*rs.Sheet{cur}, Thresholds{}); len(tl.PastParents) != 0 {
		t.Errorf("single sheet PastParents = %+v", tl.PastParents)
	}
}
