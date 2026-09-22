package site

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kwrkb/zailoop/internal/render"
	"github.com/kwrkb/zailoop/internal/rs"
)

func TestBuild(t *testing.T) {
	out := t.TempDir()
	dir := rs.Dir{Path: filepath.Join("..", "..", "testdata", "2024"), Year: 2024}
	st, err := Build(dir, out, Options{
		Now: func() time.Time { return time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if st.Projects != 4 || st.Bytes == 0 {
		t.Errorf("stats = %+v", st)
	}
	for _, f := range []string{"index.html", "list.html", "p/11.html", "p/884.html", "p/3522.html", "p/18556.html", "assets/style.css", "assets/app.js"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	detail, _ := os.ReadFile(filepath.Join(out, "p", "884.html"))
	for _, want := range []string{"法教育の推進", "40,217,000 円", "縮減", "36,000,000 円", "24,844,000 円", "3,524,000 円", render.Attribution, "2026-09-22", `href="../index.html"`, `href="../assets/style.css"`} {
		if !strings.Contains(string(detail), want) {
			t.Errorf("p/884.html missing %q", want)
		}
	}
	zero, _ := os.ReadFile(filepath.Join(out, "p", "11.html"))
	if !strings.Contains(string(zero), "算出対象外") {
		t.Errorf("p/11.html should mention 算出対象外")
	}
	index, _ := os.ReadFile(filepath.Join(out, "index.html"))
	s := string(index)
	if !strings.Contains(s, render.Attribution) {
		t.Errorf("index missing attribution")
	}
	i := strings.Index(s, `<script id="zailoop-data" type="application/json">`)
	if i < 0 {
		t.Fatal("index missing data script")
	}
	i += len(`<script id="zailoop-data" type="application/json">`)
	j := strings.Index(s[i:], "</script>")
	raw := s[i : i+j]
	var p Payload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("embedded JSON invalid: %v\n%s", err, raw[:200])
	}
	if p.Meta.Count != 4 || p.Meta.SheetYear != 2024 || p.Meta.ActualYear != 2023 || len(p.Rows) != 4 || len(p.Meta.Signals) == 0 {
		t.Errorf("payload meta = %+v rows=%d", p.Meta, len(p.Rows))
	}
	var r884 *Row
	for i := range p.Rows {
		if p.Rows[i].ID == "884" {
			r884 = &p.Rows[i]
		}
	}
	if r884 == nil || r884.Initial == nil || *r884.Initial != 23737000 || p.Meta.Ministries[r884.Ministry] != "法務省" || p.Meta.Reflections[r884.Reflection] != "縮減" || len(r884.ByYear) != 4 {
		t.Errorf("row 884 = %+v", r884)
	}
	if strings.Contains(raw, "</script>") || strings.Contains(raw, "<") {
		t.Errorf("embedded JSON must not contain raw '<'")
	}
	list, _ := os.ReadFile(filepath.Join(out, "list.html"))
	if !strings.Contains(string(list), `href="p/884.html"`) {
		t.Errorf("list.html missing link to 884")
	}
}

func TestBuildRejectsBadID(t *testing.T) {
	src := sheetSource{&rs.Sheet{FiscalYear: 2024, Project: rs.Project{ID: "../x"}}}
	if _, err := Build(src, t.TempDir(), Options{}); err == nil {
		t.Fatal("expected error for non-numeric ID")
	}
}

type sheetSource []*rs.Sheet

func (s sheetSource) Each(fn func(*rs.Sheet) error) error {
	for _, sh := range s {
		if err := fn(sh); err != nil {
			return err
		}
	}
	return nil
}
