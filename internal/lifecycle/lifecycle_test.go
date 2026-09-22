package lifecycle

import (
	"testing"

	"github.com/kwrkb/zailoop/internal/rs"
)

func y(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }

// sheet884 は docs/data-survey.md §4.3 のトレース例を手で組んだもの。
func sheet884() *rs.Sheet {
	return &rs.Sheet{
		FiscalYear: 2024,
		Project:    rs.Project{ID: "884", Name: "法教育の推進", Ministry: "法務省"},
		Budgets: []rs.BudgetYear{
			{Year: 2021, HasTotal: true, Total: rs.BudgetTotal{Initial: y(28854000), Current: y(28854000), Executed: y(23000000), CarriedOut: y(0), NextRequest: y(42300000)}},
			{Year: 2022, HasTotal: true, Total: rs.BudgetTotal{Initial: y(30261000), Supplementary: y(8220000), Current: y(38481000), Executed: y(26000000), CarriedOut: y(8220000), NextRequest: y(36000000)}},
			{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(23737000), Supplementary: y(8260000), CarriedIn: y(8220000), Reserve: y(0), Current: y(40217000), Executed: y(28433000), ExecRate: rs.Ratio{Value: 0.70699, Valid: true}, CarriedOut: y(8260000), NextRequest: y(42023000)}},
			{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(11169000), CarriedIn: y(8260000), Current: y(19429000), Executed: y(0), NextRequest: y(24844000), ChangeReason: "庁費：調査研究経費等による増"},
				Accounts: []rs.BudgetAccount{{Category: "一般会計", Demand: y(15048000)}}},
		},
		Items: []rs.BudgetItem{
			{Year: 2023, Kind: "当初予算", Amount: y(19193000)},
			{Year: 2023, Kind: "当初予算", Amount: y(4544000)},
			{Year: 2023, Kind: "第1次補正予算", Amount: y(8260000)},
			{Year: 2023, Kind: "前年度から繰越し", Amount: y(8220000)},
			{Year: 2024, Kind: "当初予算", Amount: y(11169000)},
		},
		Indicators: []rs.Indicator{
			{Number: "1", Kind: "アクティビティ", Goal: "協議会を開催"},
			{Number: "1", Kind: "アウトカム", Term: "1.短期", Goal: "学校等への支援", Metric: "出前授業の年間実施状況", Unit: "件",
				Targets: map[int]string{2023: "3000"}, Actuals: map[int]string{2023: "5319"}, Rates: map[int]string{2023: "177.3"}},
		},
		Evaluation: rs.Evaluation{
			SelfCheck: "必要性、効率性、有効性のいずれも満たしている。", Improvement: "－",
			ExternalYear: "2024", ExternalTarget: "書面点検", ExternalOpinion: "小学校での教材利用率が低い。",
			TeamOpinion: "事業内容の一部改善", Reflection: "縮減", ReflectedGeneral: y(-569000),
		},
		Blocks: []rs.PayeeBlock{
			{Number: "A", Name: "麹町税務署ほか", Total: y(1437000)},
			{Number: "B", Name: "株式会社ＩＡＣＥトラベルほか", Total: y(328000)},
			{Number: "C", Name: "株式会社バリュースほか", Total: y(26668000)},
			{Number: "D", Name: "合計なし"},
		},
	}
}

func TestBuildTrace884(t *testing.T) {
	lc := Build(sheet884(), Options{})
	if lc.ActualYear != 2023 {
		t.Fatalf("ActualYear = %d", lc.ActualYear)
	}
	if lc.Request.FiscalYear != 2023 || lc.Request.Amount.Value != 36000000 || lc.Request.State != StateOK {
		t.Errorf("Request = %+v", lc.Request)
	}
	if lc.Enacted.Current.Value != 40217000 || lc.Enacted.Supplementary.Value != 8260000 || lc.Enacted.State != StateOK {
		t.Errorf("Enacted = %+v", lc.Enacted)
	}
	if len(lc.Enacted.Items) != 3 || lc.Enacted.Items[0].Kind != "当初予算" || lc.Enacted.Items[0].Amount.Value != 23737000 || lc.Enacted.Items[0].Count != 2 {
		t.Errorf("Items = %+v", lc.Enacted.Items)
	}
	if lc.Execution.Executed.Value != 28433000 || !lc.Execution.Rate.Valid || lc.Execution.State != StateOK {
		t.Errorf("Execution = %+v", lc.Execution)
	}
	if lc.Settlement.CarriedOut.Value != 8260000 || lc.Settlement.Unused.Value != 3524000 || lc.Settlement.UnusedState != StateOK {
		t.Errorf("Settlement = %+v", lc.Settlement)
	}
	if lc.Evaluation.Improvement != "" || lc.Evaluation.TeamOpinion != "事業内容の一部改善" {
		t.Errorf("Evaluation = %+v", lc.Evaluation)
	}
	if len(lc.Evaluation.Outcomes) != 1 || lc.Evaluation.Outcomes[0].Actual != "5319" || lc.Evaluation.Outcomes[0].Rate != "177.3" {
		t.Errorf("Outcomes = %+v", lc.Evaluation.Outcomes)
	}
	r := lc.Reflection
	if r.Status != "縮減" || r.ReflectedGeneral.Value != -569000 || r.RequestYear != 2025 || r.Amount.Value != 24844000 || r.Demand.Value != 15048000 || r.State != StateOK {
		t.Errorf("Reflection = %+v", r)
	}
	if r.NextInitial.Value != 11169000 || r.NextInitialState != StateOK {
		t.Errorf("NextInitial = %+v", r)
	}
	if lc.BlockCount != 4 || len(lc.Payees) != 3 || lc.Payees[0].Number != "C" || lc.Payees[2].Number != "B" {
		t.Errorf("Payees = %+v count=%d", lc.Payees, lc.BlockCount)
	}
	if len(lc.Notes) != 0 {
		t.Errorf("Notes = %v", lc.Notes)
	}
}

func TestBuildNewProjectOnlyCurrentYear(t *testing.T) {
	s := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100), Current: y(100), Executed: y(0), NextRequest: y(200)}},
	}}
	lc := Build(s, Options{})
	if lc.Request.State != StateMissing || lc.Enacted.State != StateMissing || lc.Execution.State != StateMissing || lc.Settlement.UnusedState != StateMissing {
		t.Errorf("new project should be missing for FY2023: %+v %+v", lc.Request, lc.Enacted)
	}
	if lc.Reflection.State != StateOK || lc.Reflection.Amount.Value != 200 || lc.Reflection.NextInitial.Value != 100 {
		t.Errorf("Reflection = %+v", lc.Reflection)
	}
}

func TestBuildCurrentZeroWithExecution(t *testing.T) {
	s := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{
		{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(0), Current: y(0), Executed: y(12896926000), ExecRate: rs.Ratio{Value: 9, Valid: true}, CarriedOut: y(0)}},
	}}
	lc := Build(s, Options{})
	if lc.Execution.State != StateNotComputable || lc.Execution.Executed.Value != 12896926000 || lc.Execution.Rate.Valid {
		t.Errorf("Execution = %+v", lc.Execution)
	}
	if lc.Settlement.UnusedState != StateNotComputable {
		t.Errorf("Settlement = %+v", lc.Settlement)
	}
}

func TestBuildNegativeDiff(t *testing.T) {
	s := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{
		{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Current: y(1000), Executed: y(600), CarriedOut: y(700)}},
	}}
	lc := Build(s, Options{})
	if lc.Settlement.UnusedState != StateNeedsReview || lc.Settlement.Diff != -300 || lc.Settlement.Unused.Valid {
		t.Errorf("Settlement = %+v", lc.Settlement)
	}
}

func TestBuildNotesOnLaterExecution(t *testing.T) {
	s := &rs.Sheet{FiscalYear: 2024, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Executed: y(5)}, Issues: []string{"合計行が 2 行"}},
	}}
	lc := Build(s, Options{})
	if len(lc.Notes) != 2 {
		t.Errorf("Notes = %v", lc.Notes)
	}
}

func TestClean(t *testing.T) {
	for in, want := range map[string]string{"": "", "-": "", "ー": "", "－": "", "--": "", "ｰ": "", "―": "", " - ": "", "良好": "良好", "-a": "-a"} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}
