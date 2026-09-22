package rs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newMultiTestDir(t *testing.T, year int, ids ...string) Dir {
	t.Helper()
	rows := map[string][]csvValues{"1-2": {}}
	for _, id := range ids {
		rows["1-2"] = append(rows["1-2"], csvValues{
			"予算事業ID": id, "事業年度": strconv.Itoa(year), "事業名": id,
		})
		rows["2-1"] = append(rows["2-1"], csvValues{
			"予算事業ID": id, "予算年度": strconv.Itoa(year - 1), "当初予算（合計）": "100",
		})
	}
	dir := newTestDir(t, rows)
	if year != dir.Year {
		for _, number := range csvNumbers {
			path := filepath.Join(dir.Path, fmt.Sprintf("%s_RS_%d_test.csv", number, year))
			if err := os.Rename(testCSVPath(dir, number), path); err != nil {
				t.Fatal(err)
			}
		}
		dir.Year = year
	}
	return dir
}

func TestMultiEach(t *testing.T) {
	dir2024 := newMultiTestDir(t, 2024, "1", "2")
	dir2025 := newMultiTestDir(t, 2025, "1", "3")
	for _, dirs := range [][]Dir{{dir2024, dir2025}, {dir2025, dir2024}} {
		t.Run(fmt.Sprintf("first=%d", dirs[0].Year), func(t *testing.T) {
			original := slices.Clone(dirs)
			var got [][]*Sheet
			err := (Multi{Dirs: dirs}).Each(func(sheets []*Sheet) error {
				got = append(got, sheets)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(dirs, original) {
				t.Error("Each changed the input Dirs")
			}
			if len(got) != 3 {
				t.Fatalf("groups = %d, want 3", len(got))
			}
			for i, years := range [][]int{{2024, 2025}, {2024}, {2025}} {
				if len(got[i]) != len(years) {
					t.Fatalf("group %d: sheets = %d, want %d", i, len(got[i]), len(years))
				}
				for j, year := range years {
					sheet := got[i][j]
					id := strconv.Itoa(i + 1)
					if sheet.Project.ID != id || sheet.FiscalYear != year {
						t.Fatalf("group %d: ID = %s, year = %d; want %s, %d", i, sheet.Project.ID, sheet.FiscalYear, id, year)
					}
					dir := dir2024
					if year == 2025 {
						dir = dir2025
					}
					want, err := dir.LoadSheet(id)
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(sheet, want) {
						t.Errorf("ID %s, year %d differs from LoadSheet", id, year)
					}
				}
			}
		})
	}
}

func TestMultiSingleDirMatchesEach(t *testing.T) {
	dir := newMultiTestDir(t, 2024, "1", "2", "10")
	var want, got []*Sheet
	if err := dir.Each(func(s *Sheet) error {
		want = append(want, s)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := (Multi{Dirs: []Dir{dir}}).Each(func(sheets []*Sheet) error {
		if len(sheets) != 1 {
			t.Fatalf("sheets = %d, want 1", len(sheets))
		}
		got = append(got, sheets[0])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Error("single-dir Multi differs from Dir.Each")
	}
}

func TestMultiNumericOrder(t *testing.T) {
	dirs := []Dir{
		newMultiTestDir(t, 2025, "0", "2", "9223372036854775807"),
		newMultiTestDir(t, 2024, "-9223372036854775808", "2", "10"),
		newMultiTestDir(t, 2023),
	}
	var ids []string
	if err := (Multi{Dirs: dirs}).Each(func(sheets []*Sheet) error {
		ids = append(ids, sheets[0].Project.ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"-9223372036854775808", "0", "2", "10", "9223372036854775807"}
	if !slices.Equal(ids, want) {
		t.Fatalf("IDs = %v, want %v", ids, want)
	}
}

func TestMultiEmpty(t *testing.T) {
	for _, dirs := range [][]Dir{nil, {newMultiTestDir(t, 2024), newMultiTestDir(t, 2025)}} {
		if err := (Multi{Dirs: dirs}).Each(func([]*Sheet) error {
			t.Error("empty input must not call fn")
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMultiDuplicateYear(t *testing.T) {
	// Invalid paths ensure duplicate years are rejected before opening any CSV.
	m := Multi{Dirs: []Dir{{Year: 2024}, {Year: 2025}, {Year: 2024}}}
	err := m.Each(func([]*Sheet) error {
		t.Error("duplicate years must not call fn")
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "事業年度 2024 の Dir が重複") {
		t.Fatalf("Each = %v, want duplicate year error", err)
	}
}

func TestMultiYearMismatch(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{
		"1-2": {{"予算事業ID": "1", "事業年度": "2025"}},
	})
	check := func(sheets []*Sheet) error {
		t.Error("mismatched year must not call fn")
		return nil
	}
	want := "事業年度 2025 の CSV を年度 2024 として読もうとしています"
	for name, err := range map[string]error{
		"Multi": (Multi{Dirs: []Dir{dir}}).Each(check),
		"Dir":   dir.Each(func(s *Sheet) error { return check([]*Sheet{s}) }),
	} {
		if err == nil || err.Error() != want {
			t.Errorf("%s.Each = %v, want %s", name, err, want)
		}
	}
	sheet, err := dir.LoadSheet("1")
	if err != nil || sheet.FiscalYear != 2025 {
		t.Fatalf("LoadSheet = %+v, %v; want unchanged year handling", sheet, err)
	}
}

func TestMultiStopsOnCallbackError(t *testing.T) {
	dir2024 := newMultiTestDir(t, 2024, "1", "2")
	dir2025 := newMultiTestDir(t, 2025, "1", "3")
	writeTestCSV(t, testCSVPath(dir2024, "1-2"), fixtureHeader(t, "1-2"), []csvValues{
		{"予算事業ID": "1", "事業年度": "2024"},
		{"予算事業ID": "2", "事業年度": "invalid"},
	})
	want := errors.New("stop")
	calls := 0
	err := (Multi{Dirs: []Dir{dir2024, dir2025}}).Each(func([]*Sheet) error {
		calls++
		return want
	})
	if err != want || calls != 1 {
		t.Fatalf("Each = %v, calls = %d; want callback error and one call", err, calls)
	}
}

func TestMultiReadErrors(t *testing.T) {
	t.Run("open", func(t *testing.T) {
		dir := newMultiTestDir(t, 2025)
		path := filepath.Join(dir.Path, "5-1_RS_2025_test.csv")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		err := (Multi{Dirs: []Dir{newMultiTestDir(t, 2024, "1"), dir}}).Each(func([]*Sheet) error {
			t.Error("failed initialization must not call fn")
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "5-1_RS_2025_") {
			t.Fatalf("Each = %v, want missing CSV error", err)
		}
	})
	t.Run("unordered", func(t *testing.T) {
		dir := newMultiTestDir(t, 2024, "1", "2", "3", "2")
		calls := 0
		err := (Multi{Dirs: []Dir{dir}}).Each(func([]*Sheet) error {
			calls++
			return nil
		})
		if !errors.Is(err, ErrUnordered) || calls != 2 {
			t.Fatalf("Each = %v, calls = %d; want ErrUnordered after two calls", err, calls)
		}
	})
}

func TestMultiRealData(t *testing.T) {
	if os.Getenv("ZAILOOP_TEST_REAL_DATA") != "1" {
		t.Skip("set ZAILOOP_TEST_REAL_DATA=1 to scan data/csv")
	}
	dirs := []Dir{{Path: "../../data/csv", Year: 2024}, {Path: "../../data/csv", Year: 2025}}
	count, both := 0, 0
	byYear := make(map[int]int)
	var sheets884 []*Sheet
	var lastID int64
	start := time.Now()
	err := (Multi{Dirs: dirs}).Each(func(sheets []*Sheet) error {
		id, err := strconv.ParseInt(sheets[0].Project.ID, 10, 64)
		if err != nil {
			return err
		}
		if count > 0 && id <= lastID {
			t.Fatalf("ID %d after %d is not ascending", id, lastID)
		}
		lastID = id
		count++
		if len(sheets) == 2 {
			both++
		}
		for i, sheet := range sheets {
			if sheet.Project.ID != sheets[0].Project.ID || (i > 0 && sheet.FiscalYear <= sheets[i-1].FiscalYear) {
				t.Fatalf("ID or year order mismatch: %+v", sheet)
			}
			byYear[sheet.FiscalYear]++
		}
		if sheets[0].Project.ID == "884" {
			sheets884 = sheets
		}
		return nil
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Multi.Each: union=%d, both=%d, 2024=%d, 2025=%d in %s", count, both, byYear[2024], byYear[2025], elapsed)
	if both != 5231 {
		t.Errorf("both = %d, want 5231", both)
	}
	if len(sheets884) != 2 {
		t.Fatalf("ID 884 sheets = %d, want 2", len(sheets884))
	}
	for i, dir := range dirs {
		want, err := dir.LoadSheet("884")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sheets884[i], want) {
			t.Errorf("Multi real-data sheet 884 for %d differs from LoadSheet", dir.Year)
		}
	}
}
