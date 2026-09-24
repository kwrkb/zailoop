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
		if got := YenValue(v); got != want {
			t.Errorf("YenValue(%d) = %q, want %q", v, got, want)
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

// web/test/format.test.ts の yenShort と同じケース。
func TestYenShort(t *testing.T) {
	for _, c := range []struct {
		in   rs.Yen
		want string
	}{
		{rs.Yen{}, "—"},
		{rs.Yen{Value: 500, Valid: true}, "500 円"},
		{rs.Yen{Value: 12_345, Valid: true}, "1.2 万円"},
		{rs.Yen{Value: 40_217_000, Valid: true}, "4,022 万円"},
		{rs.Yen{Value: 123_456_789, Valid: true}, "1.2 億円"},
		{rs.Yen{Value: 2_000_000_000_000, Valid: true}, "2 兆円"},
		{rs.Yen{Value: -300_000_000, Valid: true}, "-3 億円"},
		{rs.Yen{Value: 10_000, Valid: true}, "1 万円"},
		{rs.Yen{Value: 999_950_000, Valid: true}, "10 億円"},
		{rs.Yen{Value: 150_000_000_000, Valid: true}, "1,500 億円"},
		{rs.Yen{Value: 999, Valid: true}, "999 円"},
		{rs.Yen{Value: 35_000, Valid: true}, "3.5 万円"},
		{rs.Yen{Value: 1_200_000_000, Valid: true}, "12 億円"},
		{rs.Yen{Value: 677_292_895_000, Valid: true}, "6,773 億円"},
		{rs.Yen{Value: 1_500_000_000_000, Valid: true}, "1.5 兆円"},
		{rs.Yen{Value: -569_000, Valid: true}, "-56.9 万円"},
	} {
		if got := YenShort(c.in); got != c.want {
			t.Errorf("YenShort(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// 金額がすべて 0 で、説明が基本情報の備考と 5-4 の契約にしかない事業。
func TestTextAllZeroRemarksAndObligations(t *testing.T) {
	z := rs.Yen{Valid: true}
	s := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1", Name: "x", Remarks: "2024年度執行額 1,000千円", URL: "https://example.go.jp/x"},
		Budgets:     []rs.BudgetYear{{Year: 2025, HasTotal: true, Total: rs.BudgetTotal{Initial: z, Current: z}}},
		Obligations: []rs.Obligation{{Block: "A", Payee: "契約先", Summary: "契約1", Amount: rs.Yen{Value: 5000, Valid: true}, Method: "一般競争入札"}},
	}
	var buf bytes.Buffer
	if err := Text(&buf, lifecycle.Build(s, lifecycle.Options{})); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"2025年度シートの基本情報", "2024年度執行額 1,000千円", "https://example.go.jp/x", "国庫債務負担行為等による契約", "A 契約1", "5,000 円  契約先  一般競争入札",
		"基本情報の「備考」に記載があります", "国庫債務負担行為等による契約の記載があります"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "記載はありません") {
		t.Errorf("remarks exist, so the sheet is not silent:\n%s", out)
	}

	s.Project.Remarks, s.Obligations = "", nil
	buf.Reset()
	if err := Text(&buf, lifecycle.Build(s, lifecycle.Options{})); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "「その他特記事項」「主な増減理由」「備考」に記載はありません") {
		t.Errorf("missing no-notes message:\n%s", buf.String())
	}
}
