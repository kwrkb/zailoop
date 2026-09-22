package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/rs"
)

func TestYenValue(t *testing.T) {
	for v, want := range map[int64]string{0: "0 円", 999: "999 円", 1000: "1,000 円", 40217000: "40,217,000 円", -569000: "-569,000 円"} {
		if got := yenValue(v); got != want {
			t.Errorf("yenValue(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestTextContainsStagesAndAttribution(t *testing.T) {
	y := func(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }
	s := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "884", Name: "法教育の推進", Ministry: "法務省"},
		Budgets: []rs.BudgetYear{
			{Year: 2022, HasTotal: true, Total: rs.BudgetTotal{NextRequest: y(42023000)}},
			{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(23737000), Current: y(40217000), Executed: y(28433000), ExecRate: rs.Ratio{Value: 0.70699, Valid: true}, CarriedOut: y(8260000)}},
			{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(11169000), NextRequest: y(24844000)}},
		},
		Evaluation: rs.Evaluation{TeamOpinion: "事業内容の一部改善", Reflection: "縮減", ReflectedGeneral: y(-569000)},
	}
	var buf bytes.Buffer
	if err := Text(&buf, lifecycle.Build(s, lifecycle.Options{})); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"① 要求", "42,023,000 円", "② 成立", "40,217,000 円", "③ 執行", "70.7%", "④ 決算", "3,524,000 円", "⑤ 評価", "事業内容の一部改善", "⑥ 翌年度反映", "縮減", "-569,000 円", "FY2025 概算要求", "24,844,000 円", Attribution} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}
