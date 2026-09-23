package site

import (
	"strings"
	"testing"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/rs"
)

func y(v int64) rs.Yen { return rs.Yen{Value: v, Valid: true} }

func sheetWith(t rs.BudgetTotal) *rs.Sheet {
	return &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{{Year: 2023, HasTotal: true, Total: t}}}
}

func TestWaterfall(t *testing.T) {
	// 当初 300 − 補正 20 + 繰越 120 = 現額 400。執行 250 + 繰越 50 + 不用 100
	lc := lifecycle.Build(sheetWith(rs.BudgetTotal{Initial: y(300), Supplementary: y(-20), CarriedIn: y(120), Reserve: y(0), Current: y(400),
		Executed: y(250), ExecRate: rs.Ratio{Value: 0.625, Valid: true}, CarriedOut: y(50)}), lifecycle.Options{})
	in, out, note := waterfall(lc)
	if note != "" {
		t.Errorf("note = %q", note)
	}
	wantIn := []struct {
		label, class string
		x, w         float64
	}{
		{"当初予算", "in", 0, 300.0 / 400 * 100},
		{"補正予算", "neg", 280.0 / 400 * 100, 20.0 / 400 * 100}, // 尺度は途中の最大（ここでは現額 400）
		{"前年度繰越", "in", 280.0 / 400 * 100, 120.0 / 400 * 100},
		{"歳出予算現額", "total", 0, 400.0 / 400 * 100},
	}
	if len(in) != len(wantIn) {
		t.Fatalf("in = %+v", in)
	}
	for i, w := range wantIn {
		if in[i].Label != w.label || in[i].Class != w.class || !near(in[i].X, w.x) || !near(in[i].W, w.w) {
			t.Errorf("in[%d] = %+v, want %+v", i, in[i], w)
		}
	}
	wantOut := []string{"執行額:exec", "翌年度繰越:carry", "不用相当額:unused"}
	if len(out) != len(wantOut) {
		t.Fatalf("out = %+v", out)
	}
	for i, w := range wantOut {
		if out[i].Label+":"+out[i].Class != w {
			t.Errorf("out[%d] = %+v, want %s", i, out[i], w)
		}
	}
	if last := out[2]; !near(last.X+last.W, 100) {
		t.Errorf("unused should end at current: %+v", last)
	}

	// 執行＋繰越 が現額を超える（要確認）: 不用の行はなく注記を出す
	lc = lifecycle.Build(sheetWith(rs.BudgetTotal{Initial: y(100), Current: y(100), Executed: y(90), ExecRate: rs.Ratio{Value: 0.9, Valid: true}, CarriedOut: y(30)}), lifecycle.Options{})
	_, out, note = waterfall(lc)
	if len(out) != 2 || !strings.Contains(note, "20 円 上回る") {
		t.Errorf("needs review: out=%+v note=%q", out, note)
	}

	// 現額 0 は描かない
	lc = lifecycle.Build(sheetWith(rs.BudgetTotal{Current: y(0), Executed: y(100)}), lifecycle.Options{})
	if in, out, _ := waterfall(lc); in != nil || out != nil {
		t.Errorf("zero current should not draw: %+v %+v", in, out)
	}
}

func near(a, b float64) bool { d := a - b; return d < 1e-9 && d > -1e-9 }

func TestChange(t *testing.T) {
	for _, c := range []struct {
		from, to rs.Yen
		want     string
		ok       bool
	}{
		{y(100), y(108), "+8.0%", true},
		{y(1000), y(877), "−12.3%", true},
		{y(100), y(100), "増減なし", true},
		{y(100), y(0), "−100.0%", true},
		{y(0), y(100), "", false},
		{rs.Yen{}, y(100), "", false},
	} {
		got, ok := change(c.from, c.to)
		if got != c.want || ok != c.ok {
			t.Errorf("change(%v, %v) = %q %v, want %q %v", c.from, c.to, got, ok, c.want, c.ok)
		}
	}
}

func TestDigest(t *testing.T) {
	s24 := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100_000_000), NextRequest: y(150_000_000)}},
	}, Evaluation: rs.Evaluation{Reflection: "現状通り", TeamOpinion: "現状通り"}}
	s25 := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1"}, Budgets: []rs.BudgetYear{
		{Year: 2024, HasTotal: true, Total: rs.BudgetTotal{Initial: y(100_000_000), Current: y(100_000_000), Executed: y(80_000_000), ExecRate: rs.Ratio{Value: 0.8, Valid: true}, CarriedOut: y(0)}},
		{Year: 2025, HasTotal: true, Total: rs.BudgetTotal{Initial: y(120_000_000)}},
	}, Evaluation: rs.Evaluation{Reflection: "縮減", TeamOpinion: "事業内容の一部改善"}}
	tl := lifecycle.Track([]*rs.Sheet{s24, s25}, lifecycle.Thresholds{})
	got := strings.Join(digest(tl), "\n")
	for _, want := range []string{
		"FY2024 は歳出予算現額 1 億円 のうち 8,000 万円（80.0%）を執行。不用相当額 2,000 万円。",
		"2025年度シートの推進チーム所見は「事業内容の一部改善」、翌年度要求への反映は「縮減」。",
		"FY2025 当初予算は 1.2 億円。",
		"ループ検証（このサイト独自の判定）: 2024年度シートの反映「現状通り」に対し、FY2024 当初は 1 億円 → FY2025 当初 1.2 億円（+20.0%）。この反映状況は増減を約束しないため判定対象外。",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("digest missing %q in:\n%s", want, got)
		}
	}
	// FY2024 当初がシート間で改訂されていても、増減はループ検証の基準（2024 年度シート）で 1 回だけ出す
	s25.Budgets[0].Total.Initial = y(90_000_000)
	tl = lifecycle.Track([]*rs.Sheet{s24, s25}, lifecycle.Thresholds{})
	got = strings.Join(digest(tl), "\n")
	if strings.Contains(got, "当初比") || strings.Count(got, "%）") != 2 || !strings.Contains(got, "FY2024 当初は 1 億円 → FY2025 当初 1.2 億円（+20.0%）") {
		t.Errorf("revised base should appear once:\n%s", got)
	}
	// ループ検証の文がなければ前年度比を出す
	if got := strings.Join(digest(lifecycle.Track([]*rs.Sheet{s25}, lifecycle.Thresholds{})), "\n"); !strings.Contains(got, "（FY2024 当初比 +33.3%）") {
		t.Errorf("single sheet digest:\n%s", got)
	}
	s24.Evaluation.Reflection = "廃止"
	tl = lifecycle.Track([]*rs.Sheet{s24, s25}, lifecycle.Thresholds{})
	if got := strings.Join(digest(tl), "\n"); !strings.Contains(got, "判定は「反映要確認」。「廃止」だったのに翌年度の当初予算が増えています。事業の再編・移管などの可能性もあります。") {
		t.Errorf("contradiction digest:\n%s", got)
	}
}

func TestSignalBadgesEvidence(t *testing.T) {
	lc := sheetWith(rs.BudgetTotal{Initial: y(10_000_000_000), Current: y(10_000_000_000), Executed: y(1_000_000_000), ExecRate: rs.Ratio{Value: 0.1, Valid: true}, CarriedOut: y(0)})
	tl := lifecycle.Track([]*rs.Sheet{lc}, lifecycle.Thresholds{})
	sm := lifecycle.SummarizeTimeline(tl, lifecycle.Thresholds{})
	var got []string
	for _, b := range signalBadges(tl, sm.Signals, lifecycle.Thresholds{}) {
		got = append(got, b.Label+" "+b.Evidence)
	}
	want := []string{"低執行 執行率 10%（基準 50% 未満）", "大きな不用 90 億円・現額の 90%（基準 10 億円 以上かつ 20% 以上）"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("badges = %q, want %q", got, want)
	}
}

func TestDigestSupplementaryOnly(t *testing.T) {
	tl := lifecycle.Track([]*rs.Sheet{sheetWith(rs.BudgetTotal{Initial: y(0), Supplementary: y(500000000), Current: y(500000000), Executed: y(400000000)})}, lifecycle.Thresholds{})
	got := strings.Join(digest(tl), "\n")
	if !strings.Contains(got, "FY2023 の当初予算は 0 円で、補正予算は 5 億円。") {
		t.Errorf("digest = %q", got)
	}
	if strings.Contains(got, "可能性") {
		t.Errorf("digest must not guess: %q", got)
	}
	b := newIndexBuilder(2024)
	b.add(lifecycle.SummarizeTimeline(tl, lifecycle.Thresholds{}))
	if r := b.rows[0]; r.Supp == nil || *r.Supp != 500000000 {
		t.Errorf("row sp = %v", r.Supp)
	}
}
