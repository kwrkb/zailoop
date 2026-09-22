package rs

import (
	"fmt"
	"strconv"
	"strings"
)

var indicatorColumns = func() []string {
	columns := []string{
		"アクティビティ・アウトプット・アウトカムの番号", "種別（アクティビティ・アウトプット・アウトカム）",
		"アウトカムの期間", "成果目標の種類", "アクティビティ／活動目標／成果目標", "活動指標／成果指標",
		"単位", "改善の上向き／下向き", "成果実績及び目標値の根拠として用いた統計・データ名（出典）",
		"目標年度／目標値／実績値／達成率",
	}
	for year := 2007; year <= 2060; year++ {
		columns = append(columns, strconv.Itoa(year))
	}
	return columns
}()

type indicatorBuilder struct {
	sheet *Sheet
	byKey map[[5]string]int
}

func (b *indicatorBuilder) row(r *csvRow) error {
	key := [5]string{
		r.text("アクティビティ・アウトプット・アウトカムの番号"),
		r.text("種別（アクティビティ・アウトプット・アウトカム）"),
		r.text("アウトカムの期間"), r.text("アクティビティ／活動目標／成果目標"), r.text("活動指標／成果指標"),
	}
	index, exists := b.byKey[key]
	if !exists {
		index = len(b.sheet.Indicators)
		b.byKey[key] = index
		b.sheet.Indicators = append(b.sheet.Indicators, Indicator{
			Number: key[0], Kind: key[1], Term: key[2], Goal: key[3], Metric: key[4],
			TargetType: r.text("成果目標の種類"), Unit: r.text("単位"),
			Direction:  r.text("改善の上向き／下向き"),
			Source:     r.text("成果実績及び目標値の根拠として用いた統計・データ名（出典）"),
			TargetYear: make(map[int]string), Targets: make(map[int]string),
			Actuals: make(map[int]string), Rates: make(map[int]string),
		})
	}
	indicator := &b.sheet.Indicators[index]
	var values map[int]string
	switch kind := strings.TrimSpace(r.text("目標年度／目標値／実績値／達成率")); kind {
	case "":
		return nil // Activities have no year-value rows.
	case "1.目標年度":
		values = indicator.TargetYear
	case "2.目標値":
		values = indicator.Targets
	case "3.実績値":
		values = indicator.Actuals
	case "4.達成率":
		values = indicator.Rates
	default:
		return fmt.Errorf("目標年度／目標値／実績値／達成率: 不明な行種別 %q", kind)
	}
	for year := 2007; year <= 2060; year++ {
		value := r.text(strconv.Itoa(year))
		if strings.TrimSpace(value) != "" {
			values[year] = value
		}
	}
	return nil
}

func (b *indicatorBuilder) finish() error { return nil }
