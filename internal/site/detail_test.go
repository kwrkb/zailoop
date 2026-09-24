package site

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/rs"
)

func TestDetailEscapesHTML(t *testing.T) {
	s := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1", Name: `<script>alert("x")</script>`, Ministry: "A&B"},
		Evaluation: rs.Evaluation{SelfCheck: "<b>bold</b>"}}
	tl := lifecycle.Track([]*rs.Sheet{s}, lifecycle.Thresholds{})
	var buf bytes.Buffer
	if err := writeDetail(&buf, tl, lifecycle.SummarizeTimeline(tl, lifecycle.Thresholds{}), lifecycle.Thresholds{}, nil, "now"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert") || strings.Contains(out, "<b>bold") {
		t.Errorf("unescaped HTML in output")
	}
	if !strings.Contains(out, "&lt;script&gt;") || !strings.Contains(out, "A&amp;B") {
		t.Errorf("expected escaped text, got:\n%s", out)
	}
}

func TestDetailAllZeroShowsRemarks(t *testing.T) {
	z := rs.Yen{Valid: true}
	s := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1", Name: "x", Remarks: "備考の本文", URL: "javascript:alert(1)"},
		Budgets: []rs.BudgetYear{{Year: 2025, HasTotal: true, Total: rs.BudgetTotal{Initial: z}}}}
	tl := lifecycle.Track([]*rs.Sheet{s}, lifecycle.Thresholds{})
	var buf bytes.Buffer
	if err := writeDetail(&buf, tl, lifecycle.SummarizeTimeline(tl, lifecycle.Thresholds{}), lifecycle.Thresholds{}, nil, "now"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "備考（2025年度シートの基本情報）: 備考の本文") || strings.Contains(out, "記載はありません") {
		t.Errorf("remarks should explain the zero section:\n%s", out)
	}
	if strings.Contains(out, "<summary>備考</summary>") {
		t.Errorf("remarks are already in the zero section")
	}
	if strings.Contains(out, `href="javascript:`) {
		t.Errorf("unsafe URL scheme in href")
	}
}

func TestIndexJSONEscapesScriptTerminator(t *testing.T) {
	s := &rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "1", Name: `x</script><script>alert(1)`}}
	lc := lifecycle.Build(s, lifecycle.Options{})
	ib := newIndexBuilder(2024)
	ib.add(lifecycle.Summarize(lc, lifecycle.Thresholds{}))
	var buf bytes.Buffer
	if err := writeIndex(&buf, ib.payload(2024, 2023, lifecycle.DefaultThresholds(), "now")); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), "</script>") != 2 { // データ script と app.js の 2 つだけ
		t.Errorf("unexpected </script> count in:\n%s", buf.String())
	}
}

func TestRelatedGroups(t *testing.T) {
	rels := []rs.Related{
		{ID: "1936", Name: "親", Kind: "親事業"},
		{ID: "9", Name: "外", Kind: "その他関連先"},
		{ID: "10", Name: "内", Kind: "その他関連先"},
		{ID: "../x", Name: "不正", Kind: ""},
	}
	got := relatedGroups(rels, map[string]bool{"1936": true, "10": true, "../x": true}, "../")
	if len(got) != 3 || got[0].Kind != "親事業" || got[1].Kind != "その他関連先" || got[2].Kind != "関連性の記載なし" {
		t.Fatalf("groups = %+v", got)
	}
	if got[0].Items[0].Href != "../p/1936.html" {
		t.Errorf("parent href = %q", got[0].Items[0].Href)
	}
	if got[1].Items[0].Href != "" || got[1].Items[1].Href != "../p/10.html" {
		t.Errorf("only known IDs should link: %+v", got[1].Items)
	}
	if got[2].Items[0].Href != "" {
		t.Errorf("non-numeric ID must not become a path: %+v", got[2].Items[0])
	}
}

func TestDetailObligationsCollapseOverTen(t *testing.T) {
	for n, collapsed := range map[int]bool{10: false, 11: true} {
		s := &rs.Sheet{FiscalYear: 2025, Project: rs.Project{ID: "1", Name: "x"}}
		for i := 0; i < n; i++ {
			s.Obligations = append(s.Obligations, rs.Obligation{Block: "A", Summary: "契約", Amount: rs.Yen{Value: 1, Valid: true}})
		}
		tl := lifecycle.Track([]*rs.Sheet{s}, lifecycle.Thresholds{})
		var buf bytes.Buffer
		if err := writeDetail(&buf, tl, lifecycle.SummarizeTimeline(tl, lifecycle.Thresholds{}), lifecycle.Thresholds{}, nil, "now"); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(buf.String(), "件を表示</summary>"); got != collapsed {
			t.Errorf("%d contracts: collapsed = %v, want %v", n, got, collapsed)
		}
	}
}
