package rs

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCompareHeaders2025To2026(t *testing.T) {
	diffs, err := CompareHeaders(Dir{Path: "../../testdata/2025", Year: 2025}, Dir{Path: "../../testdata/2026", Year: 2026})
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for _, d := range diffs {
		tables = append(tables, d.Table)
		if d.OnlyPrev || d.OnlyCur {
			t.Errorf("%s: file missing on one side", d.Table)
		}
	}
	// data-survey §9: 15 表中 11 表は同じで、違うのは 2-1・3-1・5-1・5-4
	if want := []string{"2-1", "3-1", "5-1", "5-4"}; !slices.Equal(tables, want) {
		t.Fatalf("tables with diffs = %v, want %v", tables, want)
	}
	if !slices.Contains(diffs[0].Removed, carriedOutColumn) {
		t.Errorf("2-1 Removed = %v, want %q", diffs[0].Removed, carriedOutColumn)
	}
	if !slices.Contains(diffs[0].Added, "当初予算（一般会計・特別会計合計）") {
		t.Errorf("2-1 Added = %v", diffs[0].Added)
	}
}

func TestMissingColumnsTestdata(t *testing.T) {
	for _, y := range []int{2024, 2025, 2026} {
		missing, err := Dir{Path: fmt.Sprintf("../../testdata/%d", y), Year: y}.MissingColumns()
		if err != nil {
			t.Fatal(err)
		}
		if len(missing) != 0 {
			t.Errorf("%d: missing %v", y, missing)
		}
	}
}

// copyDir は src の CSV を dst にコピーし、number の表だけヘッダを edit で書き換える。
func copyDir(t *testing.T, src, number string, edit func(header string) string) string {
	t.Helper()
	dst := t.TempDir()
	paths, err := filepath.Glob(filepath.Join(src, "*.csv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(filepath.Base(p), number+"_") {
			header, rest, _ := strings.Cut(string(b), "\n")
			b = []byte(edit(header) + "\n" + rest)
		}
		if err := os.WriteFile(filepath.Join(dst, filepath.Base(p)), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func TestMissingColumnsReportsDroppedRequired(t *testing.T) {
	dir := copyDir(t, "../../testdata/2025", "2-1", func(h string) string {
		return strings.Replace(h, ",執行額（合計）,", ",執行額X,", 1)
	})
	missing, err := Dir{Path: dir, Year: 2025}.MissingColumns()
	if err != nil {
		t.Fatal(err)
	}
	if got := missing["2-1"]; !slices.Equal(got, []string{"執行額（合計）"}) {
		t.Fatalf("missing = %v", missing)
	}
	if len(missing) != 1 {
		t.Fatalf("only 2-1 should be reported: %v", missing)
	}
}

func TestMissingColumnsAndCompareWithMissingFile(t *testing.T) {
	dir := copyDir(t, "../../testdata/2025", "", func(h string) string { return h })
	p, _ := filepath.Glob(filepath.Join(dir, "4-1_RS_2025_*.csv"))
	if err := os.Remove(p[0]); err != nil {
		t.Fatal(err)
	}
	missing, err := Dir{Path: dir, Year: 2025}.MissingColumns()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := missing["4-1"]; !ok || len(missing) != 1 {
		t.Fatalf("missing = %v", missing)
	}
	diffs, err := CompareHeaders(Dir{Path: "../../testdata/2025", Year: 2025}, Dir{Path: dir, Year: 2025})
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 || diffs[0].Table != "4-1" || !diffs[0].OnlyPrev {
		t.Fatalf("diffs = %+v", diffs)
	}
}

func TestCompareTableNumbers(t *testing.T) {
	got := []string{"10-1", "2-2", "1-5", "2-1", "1-1"}
	slices.SortFunc(got, compareTableNumbers)
	if want := []string{"1-1", "1-5", "2-1", "2-2", "10-1"}; !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}
