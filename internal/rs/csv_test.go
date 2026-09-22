package rs

import (
	"errors"
	"math"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestParseYen(t *testing.T) {
	for _, test := range []struct {
		input string
		want  Yen
	}{
		{"", Yen{}}, {" \t\r\n　", Yen{}},
		{"0", Yen{0, true}}, {" 34482000.0 ", Yen{34482000, true}},
		{" -569,000.00\t", Yen{-569000, true}}, {"+1,000", Yen{1000, true}},
		{"9007199254740993.0", Yen{9007199254740993, true}},
		{"9,223,372,036,854,775,807.0", Yen{math.MaxInt64, true}},
		{"-9223372036854775808.0", Yen{math.MinInt64, true}},
	} {
		t.Run(test.input, func(t *testing.T) {
			got, err := parseYen(test.input)
			if err != nil || got != test.want {
				t.Errorf("parseYen(%q) = %+v, %v; want %+v", test.input, got, err, test.want)
			}
		})
	}
	for _, input := range []string{"-", "abc", "NaN", "Inf", "1e3", "1.5", "1.", ".0", "1.0.0", "1 000", "9223372036854775808.0", "-9223372036854775809"} {
		t.Run(input, func(t *testing.T) {
			if value, err := parseYen(input); err == nil || value.Valid {
				t.Errorf("parseYen(%q) = %+v, %v; want invalid and an error", input, value, err)
			}
		})
	}
}

func TestMissingRequiredHeaders(t *testing.T) {
	for _, test := range []struct{ number, column string }{
		{"1-2", "事業名"}, {"2-1", "第5次補正予算"}, {"2-2", "予算額（歳出予算項目ごと）"},
		{"3-1", "2060"}, {"4-1", "反映額（特別会計）－反映額"}, {"5-1", "金額"}, {"5-1", "予算事業ID"},
	} {
		t.Run(test.number+test.column, func(t *testing.T) {
			dir := newTestDir(t, nil)
			header := fixtureHeader(t, test.number)
			header = slices.DeleteFunc(header, func(name string) bool { return name == test.column })
			writeTestCSV(t, testCSVPath(dir, test.number), header, nil)
			sheet, err := dir.LoadSheet("test")
			if sheet != nil || err == nil || !strings.Contains(err.Error(), "必須ヘッダ") || !strings.Contains(err.Error(), test.column) || errors.Is(err, ErrNotFound) {
				t.Fatalf("LoadSheet = %+v, %v; want missing header %q", sheet, err, test.column)
			}
		})
	}
}

func TestInvalidNumbersHaveContext(t *testing.T) {
	for _, test := range []struct{ number, column, value string }{
		{"1-2", "事業年度", "not-a-year"}, {"2-1", "予算年度", ""},
		{"2-1", "当初予算（合計）", "bad"}, {"2-1", "翌年度要求額（合計）", "1.2"},
		{"2-1", "第5次補正予算", "bad"}, {"2-1", "予備費等4", "bad"}, {"2-1", "要望額", "bad"},
		{"2-1", "執行率", "bad"}, {"2-1", "執行率", "NaN"}, {"2-1", "執行率", "+Inf"},
		{"2-2", "予算額（歳出予算項目ごと）", "bad"},
		{"2-2", "翌年度要求額（歳出予算項目ごと）", "bad"},
		{"4-1", "反映額（一般会計）", "bad"}, {"4-1", "反映額（特別会計）－反映額", "bad"},
		{"5-1", "ブロックの合計支出額", "bad"}, {"5-1", "支出先の合計支出額", "bad"}, {"5-1", "金額", "bad"},
	} {
		t.Run(test.number+test.column+test.value, func(t *testing.T) {
			row := csvValues{"予算年度": "2024", "事業年度": "2024", test.column: test.value}
			dir := newTestDir(t, map[string][]csvValues{test.number: {row}})
			sheet, err := dir.LoadSheet("test")
			if sheet != nil || err == nil {
				t.Fatalf("LoadSheet = %+v, %v; want error", sheet, err)
			}
			for _, context := range []string{test.number + "_RS_2024_test.csv:2:", "予算事業ID test", test.column} {
				if !strings.Contains(err.Error(), context) {
					t.Errorf("error %q lacks %q", err, context)
				}
			}
		})
	}
}

func TestOtherProjectsAreNotConvertedOrRetained(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{
		"2-1": {{"予算事業ID": "another", "予算年度": "invalid", "当初予算（合計）": "bad"}},
		"2-2": {{"予算事業ID": "another", "予算額（歳出予算項目ごと）": "bad"}},
		"3-1": {{"予算事業ID": "another", "目標年度／目標値／実績値／達成率": "bad"}},
		"4-1": {{"予算事業ID": "another", "反映額（一般会計）": "bad"}},
		"5-1": {{"予算事業ID": "another", "金額": "bad"}},
	})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Budgets)+len(sheet.Items)+len(sheet.Indicators)+len(sheet.Blocks) != 0 || sheet.Evaluation.ReflectedGeneral.Valid {
		t.Errorf("another project's data was retained: %+v", sheet)
	}
}

func TestCSVFileAndSyntaxErrors(t *testing.T) {
	for _, name := range []string{"missing", "ambiguous", "empty", "duplicate header", "malformed", "short record"} {
		t.Run(name, func(t *testing.T) {
			dir := newTestDir(t, nil)
			path := testCSVPath(dir, "5-1")
			switch name {
			case "missing":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "ambiguous":
				writeTestCSV(t, strings.Replace(path, "test.csv", "second.csv", 1), fixtureHeader(t, "5-1"), nil)
			case "empty":
				if err := os.WriteFile(path, nil, 0600); err != nil {
					t.Fatal(err)
				}
			case "duplicate header":
				header := append(fixtureHeader(t, "5-1"), "金額")
				writeTestCSV(t, path, header, nil)
			case "malformed", "short record":
				f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
				if err != nil {
					t.Fatal(err)
				}
				text := "test,short\n"
				if name == "malformed" {
					text = "\"unterminated quoted field\n"
				}
				_, writeErr := f.WriteString(text)
				closeErr := f.Close()
				if writeErr != nil || closeErr != nil {
					t.Fatalf("write=%v, close=%v", writeErr, closeErr)
				}
			}
			sheet, err := dir.LoadSheet("test")
			if sheet != nil || err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "5-1_RS_2024_") {
				t.Fatalf("LoadSheet = %+v, %v; want file/syntax error", sheet, err)
			}
		})
	}
}

func TestCSVQuotedBOMHeaderAndFullScan(t *testing.T) {
	dir := Dir{Path: t.TempDir(), Year: 2024}
	// Quoting the first header proves that the BOM is removed before csv.Read.
	data := "\ufeff\"予算事業ID\",value\nother,ignored\ntest,\"one,\ntwo\"\nother,ignored\ntest,last\n"
	if err := os.WriteFile(testCSVPath(dir, "1-2"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	var values []string
	err := dir.scan("1-2", "test", []string{"value"}, func(r *csvRow) error {
		values = append(values, r.text("value"))
		return nil
	})
	if err != nil || !slices.Equal(values, []string{"one,\ntwo", "last"}) {
		t.Fatalf("scan = %v, %v", values, err)
	}
}

func TestBudgetSumDoesNotOverflow(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{"2-1": {
		{"予算年度": "2024", "当初予算（合計）": "-9223372036854775808"},
		{"予算年度": "2024", "当初予算": "9223372036854775807"},
		{"予算年度": "2024", "当初予算": "1"},
	}})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	issues := sheet.Budget(2024).Issues
	if len(issues) != 1 || !strings.Contains(issues[0], "9223372036854775808") {
		t.Errorf("overflow was not reported as a mismatch: %v", issues)
	}
}

func TestInvalidPayeeHierarchy(t *testing.T) {
	for _, test := range []struct {
		name string
		rows []csvValues
	}{
		{"payee before block", []csvValues{{"支出先の合計支出額": "1"}}},
		{"contract before payee", []csvValues{{"ブロックの合計支出額": "1"}, {"金額": "1"}}},
		{"new block resets payee", []csvValues{{"ブロックの合計支出額": "1"}, {"支出先の合計支出額": "1"}, {"ブロックの合計支出額": "1"}, {"金額": "1"}}},
		{"wrong block", []csvValues{{"支出先ブロック番号": "A", "ブロックの合計支出額": "1"}, {"支出先ブロック番号": "B", "支出先の合計支出額": "1"}}},
		{"mixed levels", []csvValues{{"ブロックの合計支出額": "1", "金額": "1"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := newTestDir(t, map[string][]csvValues{"5-1": test.rows})
			if sheet, err := dir.LoadSheet("test"); sheet != nil || err == nil {
				t.Fatalf("LoadSheet = %+v, %v; want hierarchy error", sheet, err)
			}
		})
	}
}

func TestUnknownIndicatorRowKind(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{"3-1": {{"目標年度／目標値／実績値／達成率": "unknown"}}})
	if sheet, err := dir.LoadSheet("test"); sheet != nil || err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("LoadSheet = %+v, %v; want unknown kind error", sheet, err)
	}
}
