package rs

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 2026 年度シートの形式（合計列の繰り返し・0 の空欄・翌年度繰越の列なし・百分率の執行率・
// 5-1 の 1 行 1 契約・3-1 の列分割と番号なしの行種別）を、配布 CSV の抜粋で確かめる。

func fixtureDir2026() Dir { return Dir{Path: "../../testdata/2026", Year: 2026} }

func loadFixture2026(t *testing.T, id string) *Sheet {
	t.Helper()
	sheet, err := fixtureDir2026().LoadSheet(id)
	if err != nil {
		t.Fatal(err)
	}
	return sheet
}

func budgetOf(t *testing.T, s *Sheet, year int) *BudgetYear {
	t.Helper()
	b := s.Budget(year)
	if b == nil {
		t.Fatalf("FY%d がありません", year)
	}
	return b
}

func TestLoad2026Budget884(t *testing.T) {
	s := loadFixture2026(t, "884")
	if s.FiscalYear != 2026 || len(s.Budgets) != 6 || s.Budgets[0].Year != 2021 || s.Budgets[5].Year != 2026 {
		t.Fatalf("budgets = %+v", s.Budgets)
	}
	for _, b := range s.Budgets {
		if !b.HasTotal || len(b.Issues) > 0 || len(b.Accounts) != 1 {
			t.Errorf("FY%d: HasTotal=%v accounts=%d issues=%v", b.Year, b.HasTotal, len(b.Accounts), b.Issues)
		}
	}
	// 翌年度繰越は翌予算年度の「前年度から繰越し」から導く。
	fy23 := budgetOf(t, s, 2023)
	assertYen(t, "FY2023 補正", fy23.Total.Supplementary, 8260000)
	assertYen(t, "FY2023 翌年度繰越", fy23.Total.CarriedOut, 8260000)
	if !fy23.Total.CarriedOutDerived {
		t.Error("FY2023 CarriedOutDerived = false")
	}
	// 空欄は 0（2025 年度シートでは明示の 0 だった）。
	fy25 := budgetOf(t, s, 2025)
	assertYen(t, "FY2025 補正", fy25.Total.Supplementary, 0)
	assertYen(t, "FY2025 前年度繰越", fy25.Total.CarriedIn, 0)
	assertYen(t, "FY2025 翌年度繰越", fy25.Total.CarriedOut, 0)
	assertYen(t, "FY2025 執行", fy25.Total.Executed, 14036000)
	assertYen(t, "FY2025 会計別 第1次補正", fy25.Accounts[0].Supplementary[0], 0)
	// 0 と読んでも、セルが空欄だったことは残す（書かれていた当初・現額・執行は空欄ではない）。
	if fy25.Total.Blank != (BudgetBlank{}) {
		t.Errorf("FY2025 Blank = %+v, want none", fy25.Total.Blank)
	}
	// 執行率は百分率で書かれるので比率に直す。
	if r := fy25.Total.ExecRate; !r.Valid || math.Abs(r.Value-0.796) > 1e-9 {
		t.Errorf("FY2025 執行率 = %+v, want 0.796", r)
	}
	// 最後の予算年度は翌年度の行がないので繰越が分からない。執行率の空欄は 0 にしない。
	fy26 := budgetOf(t, s, 2026)
	if fy26.Total.CarriedOut.Valid || !fy26.Total.CarriedOutDerived || fy26.Total.ExecRate.Valid {
		t.Errorf("FY2026 翌年度繰越 = %+v, 執行率 = %+v; want invalid", fy26.Total.CarriedOut, fy26.Total.ExecRate)
	}
	assertYen(t, "FY2026 当初", fy26.Total.Initial, 10464000)
}

func TestLoad2026RepeatedTotals(t *testing.T) {
	// 3522 は会計別行が複数あり、合計列が各行に繰り返し載る。
	s := loadFixture2026(t, "3522")
	fy25 := budgetOf(t, s, 2025)
	if !fy25.HasTotal || len(fy25.Accounts) != 3 || len(fy25.Issues) > 0 {
		t.Fatalf("FY2025: HasTotal=%v accounts=%d issues=%v", fy25.HasTotal, len(fy25.Accounts), fy25.Issues)
	}
	assertYen(t, "FY2025 当初", fy25.Total.Initial, 161700000000)
	assertYen(t, "FY2025 補正", fy25.Total.Supplementary, 153656459250)
}

func TestLoad2026AllZeroYearAndNoBudget(t *testing.T) {
	// 11 の FY2026 は金額がすべて 0 で、会計区分だけが埋まった行になる。
	fy26 := budgetOf(t, loadFixture2026(t, "11"), 2026)
	if !fy26.HasTotal || len(fy26.Issues) > 0 {
		t.Fatalf("FY2026: HasTotal=%v issues=%v", fy26.HasTotal, fy26.Issues)
	}
	assertYen(t, "FY2026 当初", fy26.Total.Initial, 0)
	assertYen(t, "FY2026 現額", fy26.Total.Current, 0)
	if want := (BudgetBlank{Initial: true, Current: true, Executed: true}); fy26.Total.Blank != want {
		t.Errorf("FY2026 Blank = %+v, want %+v", fy26.Total.Blank, want)
	}
	// 21437 は予算がなく、2-1・2-2 とも予算年度まで空欄の 1 行だけを持つ。
	s := loadFixture2026(t, "21437")
	if len(s.Budgets) != 0 || len(s.Items) != 0 {
		t.Errorf("budgets = %+v, items = %+v; want none", s.Budgets, s.Items)
	}
}

func TestLoad2026Indicators(t *testing.T) {
	s := loadFixture2026(t, "884")
	byNumber := map[string]Indicator{}
	for _, in := range s.Indicators {
		byNumber[in.Number] = in
	}
	activity, output := byNumber["101"], byNumber["201"]
	if activity.Kind != "アクティビティ" || !strings.HasPrefix(activity.Goal, "法教育推進協議会及び部会を開催して") {
		t.Errorf("activity = %+v", activity)
	}
	if output.Kind != "アウトプット" || output.Goal != "法教育推進協議会及び部会の実施、教員対象研修への講師派遣" {
		t.Errorf("output = %+v", output)
	}
	// 行種別は「目標値」のように番号が付かない。
	if output.Targets[2025] != "2.000" || output.Actuals[2025] != "2.000" {
		t.Errorf("output targets = %v, actuals = %v", output.Targets, output.Actuals)
	}
}

func TestLoad2026Payees(t *testing.T) {
	s := loadFixture2026(t, "884")
	if len(s.Blocks) != 3 {
		t.Fatalf("blocks = %+v", s.Blocks)
	}
	a := s.Blocks[0]
	if a.Number != "A" || a.Name != "個人Ａほか" || a.Role != "委員等旅費・諸謝金" || len(a.Payees) != 10 {
		t.Fatalf("block A = %+v", a)
	}
	assertYen(t, "A 合計", a.Total, 335000)
	first := a.Payees[0]
	if first.Name != "個人Ａ" || len(first.Contracts) != 1 || first.Contracts[0].Summary != "法教育推進協議会等の出席に対する諸謝金・旅費" {
		t.Errorf("payee = %+v", first)
	}
	assertYen(t, "個人Ａ 合計", first.Total, 51000)
	assertYen(t, "個人Ａ 契約", first.Contracts[0].Amount, 51000)
	contracts := 0
	for _, p := range s.Blocks[2].Payees {
		contracts += len(p.Contracts)
	}
	if len(s.Blocks[2].Payees) != 15 || contracts != 17 {
		t.Errorf("block C: payees=%d contracts=%d; want 15, 17", len(s.Blocks[2].Payees), contracts)
	}
}

// newTestDir2026 は testdata/2026 のヘッダで、指定した行だけを持つ CSV 群を作る。
func newTestDir2026(t *testing.T, rows map[string][]csvValues) Dir {
	t.Helper()
	dir := Dir{Path: t.TempDir(), Year: 2026}
	for _, number := range csvNumbers {
		paths, err := filepath.Glob(filepath.Join(fixtureDir2026().Path, number+"_RS_2026_*.csv"))
		if err != nil || len(paths) != 1 {
			t.Fatalf("fixture %s: paths=%v, err=%v", number, paths, err)
		}
		header := readHeader(t, paths[0])
		records := rows[number]
		if number == "1-2" && records == nil {
			records = []csvValues{{"事業年度": "2026", "事業名": "test"}}
		}
		writeTestCSV(t, filepath.Join(dir.Path, number+"_RS_2026_test.csv"), header, records)
	}
	return dir
}

func readHeader(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line, _, _ := strings.Cut(strings.TrimPrefix(string(data), "\ufeff"), "\n")
	return strings.Split(strings.TrimSuffix(line, "\r"), ",")
}

func TestLoad2026ConflictingTotals(t *testing.T) {
	row := func(initial, account string) csvValues {
		return csvValues{"予算年度": "2025", "会計区分": "一般会計", "会計": "一般会計",
			"当初予算（一般会計・特別会計合計）": initial, "当初予算": account}
	}
	dir := newTestDir2026(t, map[string][]csvValues{"2-1": {row("100", "60"), row("100", "40"), row("90", "")}})
	s, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	b := s.Budget(2025)
	if b == nil || b.HasTotal || len(b.Issues) != 1 || !strings.Contains(b.Issues[0], "合計列の値が行によって異なります（2通り）") {
		t.Fatalf("budget = %+v", b)
	}
}

func TestLoad2026RepeatedTotalsBlankAndZero(t *testing.T) {
	// 繰り返しの行で、同じ合計が空欄と 0 で書かれていても 1 つの合計として扱う。
	// 空欄の印は、すべての行で空欄だった項目にだけ残す。
	row := func(account, initial string) csvValues {
		return csvValues{"予算年度": "2025", "会計区分": "一般会計", "会計": account,
			"当初予算（一般会計・特別会計合計）": initial}
	}
	dir := newTestDir2026(t, map[string][]csvValues{"2-1": {row("一般会計", ""), row("特別会計", "0")}})
	s, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	b := s.Budget(2025)
	if b == nil || !b.HasTotal || len(b.Issues) > 0 {
		t.Fatalf("budget = %+v", b)
	}
	assertYen(t, "当初", b.Total.Initial, 0)
	if want := (BudgetBlank{Current: true, Executed: true}); b.Total.Blank != want {
		t.Errorf("Blank = %+v, want %+v", b.Total.Blank, want)
	}
}

func TestLoad2026CarriedOutFromConflictingNextYear(t *testing.T) {
	row := func(year, carried string) csvValues {
		return csvValues{"予算年度": year, "会計区分": "一般会計", "会計": "一般会計",
			"計（一般会計・特別会計歳出予算現額合計）": "100", "前年度から繰越し（一般会計・特別会計合計）": carried}
	}
	dir := newTestDir2026(t, map[string][]csvValues{"2-1": {row("2025", ""), row("2026", "10"), row("2026", "20")}})
	s, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	fy25 := s.Budget(2025)
	if fy25 == nil || !fy25.HasTotal || fy25.Total.CarriedOut.Valid || !fy25.Total.CarriedOutDerived {
		t.Fatalf("FY2025 = %+v; want derived but unknown carried-out", fy25)
	}
	if fy26 := s.Budget(2026); fy26 == nil || fy26.HasTotal {
		t.Fatalf("FY2026 = %+v; want conflicting totals", fy26)
	}
}

func TestLoad2026PayeeRowWithoutPayeeTotal(t *testing.T) {
	dir := newTestDir2026(t, map[string][]csvValues{"5-1": {{
		"支出先ブロック番号": "A", "ブロックの合計支出額": "100", "支出先名": "x", "金額": "100",
	}}})
	if _, err := dir.LoadSheet("test"); err == nil || !strings.Contains(err.Error(), "支出先の合計がありません") {
		t.Fatalf("err = %v; want missing payee total", err)
	}
}

func TestMulti2026Fixture(t *testing.T) {
	m := Multi{Dirs: []Dir{fixtureDir(), {Path: "../../testdata/2025", Year: 2025}, fixtureDir2026()}}
	var got []*Sheet
	err := m.Each(func(sheets []*Sheet) error {
		if sheets[0].Project.ID == "884" {
			got = sheets
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].FiscalYear != 2024 || got[2].FiscalYear != 2026 {
		t.Fatalf("884 sheets = %d", len(got))
	}
}
