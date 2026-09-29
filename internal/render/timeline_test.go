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
	s24.Source, s25.Source = testSource(2024, "1-2"), testSource(2025, "1-2")
	tl := lifecycle.Track([]*rs.Sheet{s24, s25}, lifecycle.Thresholds{})
	var buf bytes.Buffer
	if err := Timeline(&buf, tl, Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"年度をまたぐ推移", "旧名", "新名", "2023 ", "2025 ", "ループ検証", "2024年度シート", "反映「縮減」", "反映額 -5 円", "判定: 反映要確認（「縮減」だったのに翌年度の当初予算が増えています。事業の再編・移管などの可能性もあります）", "翌年のシートがないため未検証", Attribution, "元データ: 2024年度・2025年度の配布 CSV（表 1-2）"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// 出典は最新シートの本文の後ではなく、推移・ループ検証の後に 1 回だけ出す
	if strings.Count(out, Attribution) != 1 || strings.Index(out, Attribution) < strings.Index(out, "ループ検証") {
		t.Errorf("attribution should appear once at the end:\n%s", out)
	}

	buf.Reset()
	if err := Timeline(&buf, tl, Options{SourceFiles: true}); err != nil {
		t.Fatal(err)
	}
	want := "元データ: 2024年度・2025年度の配布 CSV（表 1-2）\n  2024年度\n    1-2  1-2_RS_2024_x.csv\n  2025年度\n    1-2  1-2_RS_2025_x.csv\n" + Disclaimer + "\n"
	if !strings.HasSuffix(buf.String(), want) {
		t.Errorf("source files should come between the summary and the disclaimer:\n%s", buf.String())
	}
}
