package rs

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestEachMatchesLoadSheet(t *testing.T) {
	dir := fixtureDir()
	var sheets []*Sheet
	var ids []string
	if err := dir.Each(func(s *Sheet) error {
		sheets = append(sheets, s)
		ids = append(ids, s.Project.ID)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(ids, []string{"11", "884", "1319", "3522", "18556"}) {
		t.Fatalf("IDs = %v", ids)
	}
	// Compare after traversal to also catch reused buffers or builder state.
	for _, sheet := range sheets {
		want, err := dir.LoadSheet(sheet.Project.ID)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(sheet, want) {
			t.Errorf("Each sheet %s differs from LoadSheet", sheet.Project.ID)
		}
	}
}

func TestEachUnordered(t *testing.T) {
	for _, ids := range [][]string{{"2", "1"}, {"1", "2", "1"}, {"1", "01"}} {
		t.Run(strings.Join(ids, "-"), func(t *testing.T) {
			dir := newTestDir(t, map[string][]csvValues{
				"1-2": {
					{"予算事業ID": "1", "事業年度": "2024"},
					{"予算事業ID": "2", "事業年度": "2024"},
				},
			})
			var rows []csvValues
			for _, id := range ids {
				rows = append(rows, csvValues{"予算事業ID": id, "予算年度": "2024"})
			}
			writeTestCSV(t, testCSVPath(dir, "2-1"), fixtureHeader(t, "2-1"), rows)
			err := dir.Each(func(*Sheet) error { return nil })
			if !errors.Is(err, ErrUnordered) {
				t.Fatalf("Each = %v, want ErrUnordered", err)
			}
			context := fmt.Sprintf("2-1_RS_2024_test.csv:%d: 予算事業ID %s:", len(ids)+1, ids[len(ids)-1])
			if !strings.Contains(err.Error(), context) {
				t.Errorf("error %q lacks %q", err, context)
			}
		})
	}
}

func TestEachIDMissingFromProject(t *testing.T) {
	for _, id := range []string{"1", "3"} {
		t.Run(id, func(t *testing.T) {
			dir := newTestDir(t, map[string][]csvValues{
				"1-2": {{"予算事業ID": "2", "事業年度": "2024"}},
				"2-1": {{"予算事業ID": id, "予算年度": "2024"}},
			})
			err := dir.Each(func(s *Sheet) error {
				if s.Project.ID != "2" {
					t.Errorf("callback received missing project: %+v", s.Project)
				}
				return nil
			})
			want := fmt.Sprintf("予算事業ID %s が 1-2 にありません", id)
			if err == nil || err.Error() != want {
				t.Fatalf("Each = %v, want %s", err, want)
			}
		})
	}
}

func TestEachStopsOnCallbackError(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{
		"1-2": {
			{"予算事業ID": "1", "事業年度": "2024"},
			{"予算事業ID": "2", "事業年度": "invalid"},
		},
	})
	want := errors.New("stop")
	calls := 0
	err := dir.Each(func(*Sheet) error {
		calls++
		return want
	})
	if err != want || calls != 1 {
		t.Fatalf("Each = %v, calls = %d; want callback error and one call", err, calls)
	}
}

func TestEachInvalidID(t *testing.T) {
	for _, id := range []string{"bad", " ", "9223372036854775808"} {
		for _, afterFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%q/afterFirst=%t", id, afterFirst), func(t *testing.T) {
				rows := []csvValues{{"予算事業ID": id, "事業年度": "2024"}}
				line := 2
				if afterFirst {
					rows = append([]csvValues{{"予算事業ID": "1", "事業年度": "2024"}}, rows...)
					line++
				}
				dir := newTestDir(t, map[string][]csvValues{"1-2": rows})
				err := dir.Each(func(*Sheet) error { return nil })
				context := fmt.Sprintf("1-2_RS_2024_test.csv:%d: 予算事業ID %s:", line, strings.TrimSpace(id))
				if err == nil || !strings.Contains(err.Error(), context) || errors.Is(err, ErrUnordered) {
					t.Fatalf("Each = %v, want invalid ID with %q", err, context)
				}
			})
		}
	}
}

func TestEachEmptyAndSparseTables(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{"1-2": {}})
	if err := dir.Each(func(*Sheet) error {
		t.Error("empty CSVs must not call fn")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	ids := []string{"-9223372036854775808", "0", "2", "10", "9223372036854775807"}
	var rows []csvValues
	for _, id := range ids {
		rows = append(rows, csvValues{"予算事業ID": " " + id + " ", "事業年度": "2024"})
	}
	writeTestCSV(t, testCSVPath(dir, "1-2"), fixtureHeader(t, "1-2"), rows)
	writeTestCSV(t, testCSVPath(dir, "2-1"), fixtureHeader(t, "2-1"), []csvValues{
		{"予算事業ID": "2", "予算年度": "2024", "当初予算（合計）": "0"},
	})
	var got []string
	if err := dir.Each(func(s *Sheet) error {
		got = append(got, s.Project.ID)
		want, err := dir.LoadSheet(s.Project.ID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(s, want) {
			t.Errorf("sparse sheet %s differs from LoadSheet", s.Project.ID)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, ids) {
		t.Fatalf("IDs = %v, want %v", got, ids)
	}
}

func TestEachRowErrorContext(t *testing.T) {
	for _, number := range csvNumbers {
		t.Run(number, func(t *testing.T) {
			rows := map[string][]csvValues{
				"1-2": {
					{"予算事業ID": "1", "事業年度": "2024", "事業名": "first\nproject"},
					{"予算事業ID": "2", "事業年度": "2024"},
				},
			}
			bad := csvValues{"予算事業ID": "2"}
			switch number {
			case "1-2":
				rows[number][1]["事業年度"] = "bad"
			case "2-1", "2-2":
				bad["予算年度"] = "bad"
			case "3-1":
				bad["目標年度／目標値／実績値／達成率"] = "bad"
			case "4-1":
				bad["反映額（一般会計）"] = "bad"
			case "5-1":
				bad["金額"] = "1" // A builder error, rather than csvRow.err.
			}
			if number != "1-2" {
				rows[number] = []csvValues{bad}
			}
			dir := newTestDir(t, rows)
			_, want := dir.LoadSheet("2")
			err := dir.Each(func(*Sheet) error { return nil })
			if want == nil || err == nil || err.Error() != want.Error() {
				t.Fatalf("Each = %v, LoadSheet = %v; want identical row errors", err, want)
			}
		})
	}
}

func TestEachRealData(t *testing.T) {
	if os.Getenv("ZAILOOP_TEST_REAL_DATA") != "1" {
		t.Skip("set ZAILOOP_TEST_REAL_DATA=1 to scan data/csv")
	}
	dir := Dir{Path: "../../data/csv", Year: 2024}
	count := 0
	var sheet884 *Sheet
	start := time.Now()
	err := dir.Each(func(s *Sheet) error {
		count++
		if s.Project.ID == "884" {
			sheet884 = s
		}
		return nil
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5664 {
		t.Fatalf("count = %d, want 5664", count)
	}
	want, err := dir.LoadSheet("884")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sheet884, want) {
		t.Error("Each real-data sheet 884 differs from LoadSheet")
	}
	t.Logf("Each: %d sheets in %s (one pass per CSV)", count, elapsed)
}
