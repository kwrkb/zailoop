package rs

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrNotFound は 1-2 に指定した予算事業IDが存在しないことを表す。
var ErrNotFound = errors.New("予算事業IDが見つかりません")

// Dir は配布 CSV の格納先と事業年度を指定する。
type Dir struct {
	Path string
	Year int
}

// LoadSheet は8種類の CSV を逐次走査して、指定した事業のシートを返す。
// ID が存在しない場合は errors.Is(err, ErrNotFound) が true になる。
func (d Dir) LoadSheet(id string) (*Sheet, error) {
	sheet := new(Sheet)
	for _, t := range tables() {
		b := t.start(id, sheet)
		if err := d.scan(t.number, id, t.columns, b.row); err != nil {
			return nil, err
		}
		if err := b.finish(); err != nil {
			return nil, err
		}
		if project, ok := b.(*projectBuilder); ok && !project.found {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
		}
	}
	return sheet, nil
}

type builder interface {
	row(*csvRow) error
	finish() error
}

type table struct {
	number  string
	columns []string
	start   func(id string, s *Sheet) builder
}

func tables() []table {
	return []table{
		{"1-2", projectColumns, func(id string, s *Sheet) builder {
			return &projectBuilder{sheet: s, id: id}
		}},
		{"1-5", relatedColumns, func(_ string, s *Sheet) builder {
			return &relatedBuilder{sheet: s}
		}},
		{"2-1", budgetColumns, func(_ string, s *Sheet) builder {
			return &budgetBuilder{sheet: s, byYear: make(map[int]*BudgetYear), totalCounts: make(map[int]int)}
		}},
		{"2-2", itemColumns, func(_ string, s *Sheet) builder {
			return &itemBuilder{sheet: s}
		}},
		{"3-1", indicatorColumns, func(_ string, s *Sheet) builder {
			return &indicatorBuilder{sheet: s, byKey: make(map[[5]string]int)}
		}},
		{"4-1", evaluationColumns, func(_ string, s *Sheet) builder {
			return &evaluationBuilder{sheet: s}
		}},
		{"5-1", payeeColumns, func(_ string, s *Sheet) builder {
			return &payeeBuilder{sheet: s, blockIndex: -1, payeeIndex: -1}
		}},
		{"5-4", obligationColumns, func(_ string, s *Sheet) builder {
			return &obligationBuilder{sheet: s}
		}},
	}
}

var methodNames = []string{"直接実施", "補助", "負担", "交付", "分担金・拠出金", "その他"}

var projectColumns = func() []string {
	columns := []string{
		"事業年度", "事業名", "府省庁", "局・庁", "課", "事業の目的", "事業の概要",
		"事業区分", "事業開始年度", "事業終了（予定）年度", "主要経費", "事業概要URL", "備考",
	}
	for _, method := range methodNames {
		columns = append(columns, "実施方法ー"+method)
	}
	return columns
}()

type projectBuilder struct {
	sheet *Sheet
	id    string
	found bool
}

func (b *projectBuilder) row(r *csvRow) error {
	if !b.found {
		b.sheet.FiscalYear = r.year("事業年度")
		b.sheet.Project = Project{
			ID: b.id, Name: r.text("事業名"), Ministry: r.text("府省庁"),
			Bureau: r.text("局・庁"), Division: r.text("課"),
			Purpose: r.text("事業の目的"), Summary: r.text("事業の概要"),
			Category: r.text("事業区分"), StartYear: r.text("事業開始年度"),
			EndYear: r.text("事業終了（予定）年度"),
			URL:     strings.TrimSpace(r.text("事業概要URL")), Remarks: strings.TrimSpace(r.text("備考")),
		}
		for _, method := range methodNames {
			if strings.TrimSpace(r.text("実施方法ー"+method)) == "1" {
				b.sheet.Project.Methods = append(b.sheet.Project.Methods, method)
			}
		}
		b.found = true
	}
	expense := r.text("主要経費")
	if strings.TrimSpace(expense) != "" && !slices.Contains(b.sheet.Project.MajorExpense, expense) {
		b.sheet.Project.MajorExpense = append(b.sheet.Project.MajorExpense, expense)
	}
	return nil
}

func (b *projectBuilder) finish() error { return nil }

var relatedColumns = []string{"関連事業の事業ID", "関連事業の事業名", "関連性"}

type relatedBuilder struct{ sheet *Sheet }

func (b *relatedBuilder) row(r *csvRow) error {
	rel := Related{
		ID:   strings.TrimSpace(r.text("関連事業の事業ID")),
		Name: strings.TrimSpace(r.text("関連事業の事業名")),
		Kind: strings.TrimSpace(r.text("関連性")),
	}
	if rel != (Related{}) {
		b.sheet.Related = append(b.sheet.Related, rel)
	}
	return nil
}

func (b *relatedBuilder) finish() error { return nil }
