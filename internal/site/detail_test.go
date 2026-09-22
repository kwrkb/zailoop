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
	if err := writeDetail(&buf, tl, lifecycle.SummarizeTimeline(tl, lifecycle.Thresholds{}), lifecycle.Thresholds{}, "now"); err != nil {
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
