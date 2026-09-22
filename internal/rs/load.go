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

// LoadSheet は6種類の CSV を逐次走査して、指定した事業のシートを返す。
// ID が存在しない場合は errors.Is(err, ErrNotFound) が true になる。
func (d Dir) LoadSheet(id string) (*Sheet, error) {
	sheet := new(Sheet)
	found, err := d.loadProject(id, sheet)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, id)
	}
	for _, load := range []func(string, *Sheet) error{
		d.loadBudgets, d.loadItems, d.loadIndicators, d.loadEvaluation, d.loadPayees,
	} {
		if err := load(id, sheet); err != nil {
			return nil, err
		}
	}
	return sheet, nil
}

var methodNames = []string{"直接実施", "補助", "負担", "交付", "分担金・拠出金", "その他"}

func (d Dir) loadProject(id string, sheet *Sheet) (bool, error) {
	columns := []string{
		"事業年度", "事業名", "府省庁", "局・庁", "課", "事業の目的", "事業の概要",
		"事業区分", "事業開始年度", "事業終了（予定）年度", "主要経費",
	}
	for _, method := range methodNames {
		columns = append(columns, "実施方法ー"+method)
	}
	found := false
	err := d.scan("1-2", id, columns, func(r *csvRow) error {
		if !found {
			sheet.FiscalYear = r.year("事業年度")
			sheet.Project = Project{
				ID: id, Name: r.text("事業名"), Ministry: r.text("府省庁"),
				Bureau: r.text("局・庁"), Division: r.text("課"),
				Purpose: r.text("事業の目的"), Summary: r.text("事業の概要"),
				Category: r.text("事業区分"), StartYear: r.text("事業開始年度"),
				EndYear: r.text("事業終了（予定）年度"),
			}
			for _, method := range methodNames {
				if strings.TrimSpace(r.text("実施方法ー"+method)) == "1" {
					sheet.Project.Methods = append(sheet.Project.Methods, method)
				}
			}
			found = true
		}
		expense := r.text("主要経費")
		if strings.TrimSpace(expense) != "" && !slices.Contains(sheet.Project.MajorExpense, expense) {
			sheet.Project.MajorExpense = append(sheet.Project.MajorExpense, expense)
		}
		return nil
	})
	return found, err
}
