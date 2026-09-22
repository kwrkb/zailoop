package site

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kwrkb/zailoop/internal/render"
)

func TestTerms(t *testing.T) {
	ids := map[string]bool{}
	for _, tm := range Terms {
		if tm.ID == "" || tm.Name == "" || tm.Desc == "" || ids[tm.ID] {
			t.Errorf("bad or duplicate term %+v", tm)
		}
		ids[tm.ID] = true
	}
	if id, err := termID("歳出予算現額"); err != nil || id != "current" {
		t.Errorf("termID = %q, %v", id, err)
	}
	if _, err := termID("存在しない用語"); err == nil {
		t.Error("unknown term should be an error")
	}
	var buf bytes.Buffer
	if err := writeTerms(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `<dt id="unused">不用相当額</dt>`) {
		t.Error("terms.html missing anchor for 不用相当額")
	}
	for _, want := range []string{"決算上の不用額そのものではない", render.Attribution, render.Disclaimer} {
		if !strings.Contains(visibleText(buf.String()), want) {
			t.Errorf("terms.html missing %q", want)
		}
	}
}
