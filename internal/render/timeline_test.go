package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/rs"
)

func TestTimelineOutput(t *testing.T) {
	y := func(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }
	s24 := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1", Name: "旧名"}, Budgets: []rs.BudgetYear{
		{Year: 2023, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100), Current: y(100), Executed: y(90), NextRequest: y(120)}},
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(110), NextRequest: y(150)}},
	}, Evaluation: rs.Evaluation{Reflection: "縮減", ReflectedGeneral: y(-5)}}
	s25 := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1", Name: "新名"}, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(110), Current: y(110), Executed: y(70), ExecRate: rs.Ratio{Value: 0.636, Valid: true}}},
		{Year: 2025, HasTotal: true, Total: rs.BudgetTotal{Initial: y(130)}},
	}}
	tl := lifecycle.Track([]*rs.Sheet{s24, s25}, lifecycle.Thresholds{})
	var buf bytes.Buffer
	if err := Timeline(&buf, tl); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"年度をまたぐ推移", "旧名", "新名", "2023 ", "2025 ", "ループ検証", "2024年度シート", "反映「縮減」", "反映額 -5 円", "判定: 反映要確認（「縮減」だったのに翌年度の当初予算が増えています。事業の再編・移管などの可能性もあります）", "翌年のシートがないため未検証", Attribution} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
