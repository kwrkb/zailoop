package rs

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

var csvNumbers = []string{"1-2", "1-5", "2-1", "2-2", "3-1", "4-1", "5-1"}

func fixtureDir() Dir { return Dir{Path: "../../testdata/2024", Year: 2024} }

func assertYen(t *testing.T, name string, got Yen, want int64) {
	t.Helper()
	if got != (Yen{Value: want, Valid: true}) {
		t.Errorf("%s = %+v, want %d (valid)", name, got, want)
	}
}

func assertSheet884(t *testing.T, sheet *Sheet) {
	t.Helper()
	if sheet.FiscalYear != 2024 || sheet.Project.ID != "884" || sheet.Project.Name != "法教育の推進" || sheet.Project.Ministry != "法務省" {
		t.Fatalf("unexpected project: year=%d, %+v", sheet.FiscalYear, sheet.Project)
	}
	if !slices.Equal(sheet.Project.Methods, []string{"直接実施"}) || !slices.Equal(sheet.Project.MajorExpense, []string{"その他の事項経費"}) {
		t.Errorf("unexpected methods/expenses: %+v", sheet.Project)
	}
	wants := []struct {
		year                                                            int
		initial, supplementary, carriedIn, current, executed, out, next int64
		rate                                                            Ratio
	}{
		{2021, 28854000, 0, 0, 28854000, 23000000, 0, 42300000, Ratio{0.79712, true}},
		{2022, 30261000, 8220000, 0, 38481000, 26000000, 8220000, 36000000, Ratio{0.67566, true}},
		{2023, 23737000, 8260000, 8220000, 40217000, 28433000, 8260000, 42023000, Ratio{0.70699, true}},
		{2024, 11169000, 0, 8260000, 19429000, 0, 0, 24844000, Ratio{}},
	}
	if len(sheet.Budgets) != len(wants) {
		t.Fatalf("budgets = %d, want %d", len(sheet.Budgets), len(wants))
	}
	for i, want := range wants {
		budget := sheet.Budgets[i]
		if budget.Year != want.year || !budget.HasTotal || len(budget.Accounts) != 1 || len(budget.Issues) != 0 {
			t.Fatalf("unexpected budget: %+v", budget)
		}
		total := budget.Total
		assertYen(t, "initial", total.Initial, want.initial)
		assertYen(t, "supplementary", total.Supplementary, want.supplementary)
		assertYen(t, "carried in", total.CarriedIn, want.carriedIn)
		assertYen(t, "reserve", total.Reserve, 0)
		assertYen(t, "current", total.Current, want.current)
		assertYen(t, "executed", total.Executed, want.executed)
		assertYen(t, "carried out", total.CarriedOut, want.out)
		assertYen(t, "next request", total.NextRequest, want.next)
		if total.ExecRate != want.rate {
			t.Errorf("%d execution rate = %+v, want %+v", want.year, total.ExecRate, want.rate)
		}
	}
	assertYen(t, "2024 demand", sheet.Budget(2024).Accounts[0].Demand, 15048000)
	if len(sheet.Items) != 20 {
		t.Fatalf("items = %d, want 20", len(sheet.Items))
	}
	if item := sheet.Items[0]; item.Year != 2024 || item.Kind != "前年度から繰越し" || item.NextRequest.Valid {
		t.Errorf("first item = %+v", item)
	}
	assertYen(t, "first item amount", sheet.Items[0].Amount, 8260000)
	var nextItemTotal int64
	for _, item := range sheet.Items {
		if item.Year == 2024 && item.NextRequest.Valid {
			nextItemTotal += item.NextRequest.Value
		}
	}
	if nextItemTotal != 24844000 {
		t.Errorf("next request items total = %d", nextItemTotal)
	}
	if len(sheet.Indicators) != 8 {
		t.Fatalf("indicators = %d, want 8", len(sheet.Indicators))
	}
	activities, matching := 0, 0
	for _, indicator := range sheet.Indicators {
		if indicator.Kind == "アクティビティ" {
			activities++
		}
		if indicator.Metric == "出前授業の年間実施状況" {
			matching++
			if indicator.Actuals[2023] != "5319" || indicator.Targets[2023] != "3000" || indicator.Rates[2023] != "177.3" {
				t.Errorf("unexpected indicator: %+v", indicator)
			}
		}
	}
	if activities != 2 || matching != 2 {
		t.Errorf("activities=%d, matching outcomes=%d, want 2 each", activities, matching)
	}
	assertYen(t, "reflected general", sheet.Evaluation.ReflectedGeneral, -569000)
	if sheet.Evaluation.Reflection != "縮減" {
		t.Errorf("reflection = %q", sheet.Evaluation.Reflection)
	}
	if len(sheet.Blocks) != 3 {
		t.Fatalf("blocks = %d, want 3", len(sheet.Blocks))
	}
	for i, want := range []struct {
		number string
		total  int64
		payees int
	}{{"A", 1437000, 11}, {"B", 328000, 7}, {"C", 26668000, 11}} {
		block := sheet.Blocks[i]
		if block.Number != want.number || len(block.Payees) != want.payees {
			t.Fatalf("block %s: number=%s, payees=%d", want.number, block.Number, len(block.Payees))
		}
		assertYen(t, "block "+block.Number, block.Total, want.total)
		for _, payee := range block.Payees {
			if len(payee.Contracts) != 1 {
				t.Fatalf("payee %s: contracts=%d, want 1", payee.Name, len(payee.Contracts))
			}
			if payee.Contracts[0].Amount != payee.Total {
				t.Errorf("payee %s: contract=%+v, total=%+v", payee.Name, payee.Contracts[0].Amount, payee.Total)
			}
		}
	}
	if got := sheet.Blocks[1].Payees[0].CorporateNumber; got != "7010001128717" {
		t.Errorf("corporate number = %q", got)
	}
}

func TestLoadSheet884(t *testing.T) {
	sheet, err := fixtureDir().LoadSheet("884")
	if err != nil {
		t.Fatal(err)
	}
	assertSheet884(t, sheet)
}

func TestLoadSheetOtherProjects(t *testing.T) {
	for _, id := range []string{"11", "3522", "18556"} {
		t.Run(id, func(t *testing.T) {
			sheet, err := fixtureDir().LoadSheet(id)
			if err != nil {
				t.Fatal(err)
			}
			if sheet.Project.ID != id {
				t.Fatalf("ID = %q", sheet.Project.ID)
			}
			switch id {
			case "11":
				budget := sheet.Budget(2023)
				if budget == nil || !budget.Total.Current.Valid || budget.Total.Current.Value != 0 || budget.Total.Executed.Value <= 0 {
					t.Errorf("unexpected 2023 budget: %+v", budget)
				}
			case "3522":
				budget := sheet.Budget(2023)
				if budget == nil || len(budget.Accounts) < 2 {
					t.Fatalf("expected multiple accounts: %+v", budget)
				}
				categories := make(map[string]bool)
				for _, account := range budget.Accounts {
					categories[account.Category] = true
				}
				if !categories["一般会計"] || !categories["特別会計"] {
					t.Errorf("account categories = %v", categories)
				}
			case "18556":
				if len(sheet.Budgets) != 1 || sheet.Budgets[0].Year != 2024 {
					t.Errorf("expected only FY2024: %+v", sheet.Budgets)
				}
			}
			for _, budget := range sheet.Budgets {
				if !budget.HasTotal || len(budget.Issues) != 0 {
					t.Errorf("unexpected budget issues: %+v", budget)
				}
			}
		})
	}
}

func TestLoadSheetNotFound(t *testing.T) {
	sheet, err := fixtureDir().LoadSheet("99999999")
	if sheet != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("LoadSheet = %+v, %v; want nil, ErrNotFound", sheet, err)
	}
}

type csvValues = map[string]string

func fixtureHeader(t *testing.T, number string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(fixtureDir().Path, number+"_RS_2024_*.csv"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("fixture %s: paths=%v, err=%v", number, paths, err)
	}
	f, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	header, err := csv.NewReader(f).Read()
	if err != nil {
		t.Fatal(err)
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	return header
}

func testCSVPath(dir Dir, number string) string {
	return filepath.Join(dir.Path, number+"_RS_2024_test.csv")
}

func writeTestCSV(t *testing.T, path string, header []string, rows []csvValues) {
	t.Helper()
	var buf bytes.Buffer
	buf.WriteString("\ufeff")
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		values := make([]string, len(header))
		for i, name := range header {
			values[i] = row[name]
			if name == "予算事業ID" && values[i] == "" {
				values[i] = "test"
			}
		}
		if err := w.Write(values); err != nil {
			t.Fatal(err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

// Synthetic files use the publisher's headers in reverse order, so every test
// also exercises header-based lookup independently of the fixture column order.
func newTestDir(t *testing.T, rows map[string][]csvValues) Dir {
	t.Helper()
	dir := Dir{Path: t.TempDir(), Year: 2024}
	for _, number := range csvNumbers {
		header := fixtureHeader(t, number)
		slices.Reverse(header)
		records := rows[number]
		if number == "1-2" && records == nil {
			records = []csvValues{{"事業年度": "2024", "事業名": "test"}}
		}
		writeTestCSV(t, testCSVPath(dir, number), header, records)
	}
	return dir
}

func TestBudgetMismatch(t *testing.T) {
	total := csvValues{
		"予算年度": "2023", "会計区分": "一般会計", // Classification must ignore the category.
		"当初予算（合計）": "101", "補正予算（合計）": "16", "前年度からの繰越し（合計）": "8",
		"予備費等（合計）": "11", "計（歳出予算現額合計）": "151", "執行額（合計）": "91", "翌年度要求額（合計）": "201.0",
	}
	account := csvValues{
		"予算年度": "2023", "当初予算": "30", "第1次補正予算": "1", "第2次補正予算": "2",
		"第3次補正予算": "3", "第4次補正予算": "4", "第5次補正予算": "5", "前年度から繰越し": "7",
		"予備費等1": "1", "予備費等2": "2", "予備費等3": "3", "予備費等4": "4",
		"歳出予算現額": "150", "執行額": "90", "翌年度要求額": "200", "要望額": "999",
	}
	other := csvValues{"予算年度": "2023", "当初予算": "70"}
	dir := newTestDir(t, map[string][]csvValues{"2-1": {total, account, other}})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	b := sheet.Budget(2023)
	if b == nil || !b.HasTotal || len(b.Accounts) != 2 || len(b.Issues) != 7 {
		t.Fatalf("budget = %+v, want two accounts and seven mismatches", b)
	}
	for _, name := range []string{"当初予算", "補正予算", "前年度からの繰越し", "予備費等", "歳出予算現額", "執行額", "翌年度要求額"} {
		if !slices.ContainsFunc(b.Issues, func(issue string) bool { return strings.Contains(issue, name) && strings.Contains(issue, "不一致") }) {
			t.Errorf("missing %s mismatch: %v", name, b.Issues)
		}
	}
	assertYen(t, "published initial", b.Total.Initial, 101)
	assertYen(t, "published next request", b.Total.NextRequest, 201)

	// The full supplementary/reserve arrays must contribute to the comparison;
	// demand is separate from the next request and must not be added to it.
	for name, value := range map[string]string{
		"当初予算（合計）": "100", "補正予算（合計）": "15", "前年度からの繰越し（合計）": "7",
		"予備費等（合計）": "10", "計（歳出予算現額合計）": "150", "執行額（合計）": "90", "翌年度要求額（合計）": "200",
	} {
		total[name] = value
	}
	writeTestCSV(t, testCSVPath(dir, "2-1"), fixtureHeader(t, "2-1"), []csvValues{total, account, other})
	sheet, err = dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	if issues := sheet.Budget(2023).Issues; len(issues) != 0 {
		t.Errorf("matching totals: %v", issues)
	}
}

func TestBudgetMissingDuplicateAndEmptyRows(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{"2-1": {
		{"予算年度": "2024", "当初予算（合計）": "10"},
		{"予算年度": "2024", "当初予算（合計）": "20"},
		{"予算年度": "2024", "当初予算": "10"},
		{"予算年度": "2023", "当初予算": "0"},
		{"予算年度": "2022", "会計区分": "一般会計", "当初予算": "  "},
		{"予算年度": "2021", "当初予算（合計）": "0"},
		{"予算年度": "2021", "当初予算": "0"},
		{"予算年度": "2021"},
	}})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Budgets) != 4 {
		t.Fatalf("budgets = %+v", sheet.Budgets)
	}
	for i, want := range []struct {
		year, accounts, issues int
		hasTotal               bool
	}{{2021, 1, 1, true}, {2022, 0, 2, false}, {2023, 1, 1, false}, {2024, 1, 1, false}} {
		b := sheet.Budgets[i]
		if b.Year != want.year || len(b.Accounts) != want.accounts || len(b.Issues) != want.issues || b.HasTotal != want.hasTotal {
			t.Errorf("budget = %+v, want %+v", b, want)
		}
	}
	assertYen(t, "first duplicate total", sheet.Budget(2024).Total.Initial, 10)
	if sheet.Budget(2023).Total.Initial.Valid {
		t.Error("missing total was synthesized from accounts")
	}
}

func TestProjectAndEvaluationDeduplication(t *testing.T) {
	project := csvValues{
		"事業年度": "2024", "事業名": "テスト", "事業の目的": "引用,と\n改行\"を保持",
		"主要経費": "経費A", "実施方法ー直接実施": "1", "実施方法ー補助": "0", "実施方法ー負担": "1",
	}
	secondProject := maps.Clone(project)
	secondProject["主要経費"] = "経費B"
	evaluation := csvValues{
		"事業所管部局による点検・改善ー点検結果": "最初の評価", "反映額（一般会計）": "-569000",
		"過去に受けた指摘事項－区分": "検査", "過去に受けた指摘事項－取りまとめ年度": "2023",
		"過去に受けた指摘事項－取りまとめ内容": "内容1", "過去に受けた指摘事項－対応状況": "対応中",
		"その他の指摘事項－調査等の名称": "調査", "その他の指摘事項－指摘内容": "内容2",
		"反映額（特別会計）－会計": "会計A", "反映額（特別会計）－勘定": "勘定A", "反映額（特別会計）－反映額": "-10.0",
	}
	secondEvaluation := maps.Clone(evaluation)
	secondEvaluation["事業所管部局による点検・改善ー点検結果"] = "後の評価"
	secondEvaluation["反映額（一般会計）"] = "-1"
	secondEvaluation["過去に受けた指摘事項－取りまとめ内容"] = "内容3"
	secondEvaluation["その他の指摘事項－指摘内容"] = "内容4"
	secondEvaluation["反映額（特別会計）－反映額"] = "-20"
	dir := newTestDir(t, map[string][]csvValues{
		"1-2": {project, secondProject, project},
		"4-1": {evaluation, secondEvaluation, evaluation, {}},
	})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sheet.Project.MajorExpense, []string{"経費A", "経費B"}) || !slices.Equal(sheet.Project.Methods, []string{"直接実施", "負担"}) {
		t.Errorf("project = %+v", sheet.Project)
	}
	if sheet.Project.Purpose != project["事業の目的"] {
		t.Errorf("purpose = %q", sheet.Project.Purpose)
	}
	e := sheet.Evaluation
	if e.SelfCheck != "最初の評価" || len(e.PastRemarks) != 2 || len(e.OtherRemarks) != 2 || len(e.ReflectedSpecial) != 2 {
		t.Fatalf("evaluation = %+v", e)
	}
	assertYen(t, "first general reflection", e.ReflectedGeneral, -569000)
	assertYen(t, "first special reflection", e.ReflectedSpecial[0].Amount, -10)
	assertYen(t, "second special reflection", e.ReflectedSpecial[1].Amount, -20)
	if e.PastRemarks[0].Content != "内容1" || e.PastRemarks[1].Content != "内容3" || e.OtherRemarks[1].Content != "内容4" {
		t.Errorf("remark order was lost: %+v", e)
	}
}

func TestIndicatorGroupingAndRawValues(t *testing.T) {
	base := csvValues{
		"アクティビティ・アウトプット・アウトカムの番号": "1", "種別（アクティビティ・アウトプット・アウトカム）": "アウトカム",
		"アウトカムの期間": "長期", "アクティビティ／活動目標／成果目標": "目標", "活動指標／成果指標": "指標",
		"成果目標の種類": "定量的", "単位": "件", "改善の上向き／下向き": "上向き",
		"成果実績及び目標値の根拠として用いた統計・データ名（出典）": "出典",
	}
	var rows []csvValues
	for i, kind := range []string{"1.目標年度", "2.目標値", "3.実績値", "4.達成率"} {
		row := maps.Clone(base)
		row["目標年度／目標値／実績値／達成率"] = kind
		row["2007"] = []string{"1", "-", " 文字列 ", "12.3"}[i]
		row["2060"] = "1"
		rows = append(rows, row)
	}
	for _, key := range []string{
		"アクティビティ・アウトプット・アウトカムの番号", "種別（アクティビティ・アウトプット・アウトカム）",
		"アウトカムの期間", "アクティビティ／活動目標／成果目標", "活動指標／成果指標",
	} {
		row := maps.Clone(rows[0])
		row[key] += "別"
		rows = append(rows, row)
	}
	activity := csvValues{"種別（アクティビティ・アウトプット・アウトカム）": "アクティビティ", "アクティビティ／活動目標／成果目標": "活動"}
	rows = append(rows, activity, activity)
	dir := newTestDir(t, map[string][]csvValues{"3-1": rows})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Indicators) != 7 {
		t.Fatalf("indicators=%d, want 7", len(sheet.Indicators))
	}
	i := sheet.Indicators[0]
	if i.Term != "長期" || i.TargetYear[2007] != "1" || i.Targets[2007] != "-" || i.Actuals[2007] != " 文字列 " || i.Rates[2007] != "12.3" {
		t.Errorf("indicator=%+v", i)
	}
	for _, values := range []map[int]string{i.TargetYear, i.Targets, i.Actuals, i.Rates} {
		if len(values) != 2 || values[2060] != "1" {
			t.Errorf("year values=%v", values)
		}
	}
}

func TestPayeeHierarchy(t *testing.T) {
	dir := newTestDir(t, map[string][]csvValues{"5-1": {
		{"支出先ブロック番号": "A", "ブロックの合計支出額": " 1,000.0 "},
		{"支出先ブロック番号": "A", "支出先名": "同じ名前", "法人番号": " 0012345678901 ", "支出先の合計支出額": " 1000 "},
		{"支出先ブロック番号": "A", "金額": " 300 ", "契約概要": "契約1"},
		{"支出先ブロック番号": "A", "金額": " 700 ", "契約概要": "契約2"},
		{"支出先ブロック番号": "B", "ブロックの合計支出額": "0"},
		{"支出先ブロック番号": "B", "支出先名": "同じ名前", "支出先の合計支出額": "0"},
		{"支出先ブロック番号": "B", "金額": "0"},
		{},
	}})
	sheet, err := dir.LoadSheet("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Blocks) != 2 || len(sheet.Blocks[0].Payees) != 1 || len(sheet.Blocks[1].Payees) != 1 {
		t.Fatalf("blocks=%+v", sheet.Blocks)
	}
	p := sheet.Blocks[0].Payees[0]
	if p.CorporateNumber != "0012345678901" || len(p.Contracts) != 2 || p.Contracts[1].Summary != "契約2" {
		t.Errorf("payee=%+v", p)
	}
	assertYen(t, "block A", sheet.Blocks[0].Total, 1000)
	assertYen(t, "first contract", p.Contracts[0].Amount, 300)
	assertYen(t, "block B", sheet.Blocks[1].Total, 0)
	if len(sheet.Blocks[1].Payees[0].Contracts) != 1 {
		t.Error("zero-valued contract was lost")
	}
}

func TestLoadSheetRealData(t *testing.T) {
	if os.Getenv("ZAILOOP_TEST_REAL_DATA") != "1" {
		t.Skip("set ZAILOOP_TEST_REAL_DATA=1 to scan data/csv")
	}
	start := time.Now()
	sheet, err := (Dir{Path: "../../data/csv", Year: 2024}).LoadSheet("884")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	assertSheet884(t, sheet)
	fixture, err := fixtureDir().LoadSheet("884")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sheet, fixture) {
		t.Error("real-data sheet differs from the fixture sheet")
	}
	t.Logf("LoadSheet(884): %s (six complete CSV scans)", elapsed)
}

func ExampleDir_LoadSheet() {
	sheet, err := (Dir{Path: "../../testdata/2024", Year: 2024}).LoadSheet("884")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(sheet.Project.Name, sheet.Budget(2023).Total.Executed.Value)
	// Output: 法教育の推進 28433000
}

func TestLoadSheetRelated(t *testing.T) {
	dir := Dir{Path: "../../testdata/2025", Year: 2025}
	sheet, err := dir.LoadSheet("1937")
	if err != nil {
		t.Fatal(err)
	}
	want := []Related{{ID: "1936", Name: "医療提供体制推進事業", Kind: "親事業"}}
	if !slices.Equal(sheet.Related, want) {
		t.Errorf("1937 Related = %+v, want %+v", sheet.Related, want)
	}
	// 関連事業のない事業は空欄 1 行だけを持つ
	sheet, err = dir.LoadSheet("884")
	if err != nil {
		t.Fatal(err)
	}
	if len(sheet.Related) != 0 {
		t.Errorf("884 Related = %+v, want none", sheet.Related)
	}
	ids, err := dir.IDs()
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 7 || !ids["1937"] || ids["1936"] {
		t.Errorf("IDs = %v", ids)
	}
}
