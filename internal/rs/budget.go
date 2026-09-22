package rs

import (
	"cmp"
	"fmt"
	"math/big"
	"slices"
)

var totalColumns = []string{
	"当初予算（合計）", "補正予算（合計）", "前年度からの繰越し（合計）", "予備費等（合計）",
	"計（歳出予算現額合計）", "執行額（合計）", "執行率", "翌年度への繰越し(合計）", "翌年度要求額（合計）",
}

var accountColumns = []string{
	"当初予算", "第1次補正予算", "第2次補正予算", "第3次補正予算", "第4次補正予算", "第5次補正予算",
	"前年度から繰越し", "予備費等1", "予備費等2", "予備費等3", "予備費等4",
	"歳出予算現額", "執行額", "翌年度要求額", "要望額",
}

var budgetColumns = func() []string {
	columns := []string{"予算年度", "主な増減理由", "その他特記事項", "会計区分", "会計", "勘定", "備考"}
	columns = append(columns, totalColumns...)
	return append(columns, accountColumns...)
}()

type budgetBuilder struct {
	sheet       *Sheet
	byYear      map[int]*BudgetYear
	totalCounts map[int]int
}

func (b *budgetBuilder) row(r *csvRow) error {
	year := r.year("予算年度")
	budget := b.byYear[year]
	if budget == nil {
		budget = &BudgetYear{Year: year}
		b.byYear[year] = budget
	}
	hasTotal, hasAccount := r.hasAny(totalColumns), r.hasAny(accountColumns)
	if !hasTotal && !hasAccount {
		budget.Issues = append(budget.Issues, "合計列・会計別列がともに空の行を無視しました")
		return nil
	}
	if hasTotal {
		total := BudgetTotal{
			Initial: r.yen("当初予算（合計）"), Supplementary: r.yen("補正予算（合計）"),
			CarriedIn: r.yen("前年度からの繰越し（合計）"), Reserve: r.yen("予備費等（合計）"),
			Current: r.yen("計（歳出予算現額合計）"), Executed: r.yen("執行額（合計）"),
			ExecRate: r.ratio("執行率"), CarriedOut: r.yen("翌年度への繰越し(合計）"),
			NextRequest:  r.yen("翌年度要求額（合計）"),
			ChangeReason: r.text("主な増減理由"), Notes: r.text("その他特記事項"),
		}
		if b.totalCounts[year] == 0 {
			budget.Total = total
		}
		b.totalCounts[year]++
	}
	if hasAccount {
		account := BudgetAccount{
			Category: r.text("会計区分"), Account: r.text("会計"), Subaccount: r.text("勘定"),
			Initial: r.yen("当初予算"), CarriedIn: r.yen("前年度から繰越し"),
			Current: r.yen("歳出予算現額"), Executed: r.yen("執行額"),
			NextRequest: r.yen("翌年度要求額"), Demand: r.yen("要望額"), Notes: r.text("備考"),
		}
		for i := range account.Supplementary {
			account.Supplementary[i] = r.yen(fmt.Sprintf("第%d次補正予算", i+1))
		}
		for i := range account.Reserve {
			account.Reserve[i] = r.yen(fmt.Sprintf("予備費等%d", i+1))
		}
		budget.Accounts = append(budget.Accounts, account)
	}
	return nil
}

func (b *budgetBuilder) finish() error {
	for year, budget := range b.byYear {
		count := b.totalCounts[year]
		budget.HasTotal = count == 1
		switch {
		case count == 0:
			budget.Issues = append(budget.Issues, "合計行がありません（0行）")
		case count > 1:
			budget.Issues = append(budget.Issues, fmt.Sprintf("合計行が重複しています（%d行）。最初の行を保持しました", count))
		}
		if count > 0 {
			checkBudgetTotals(budget)
		}
		b.sheet.Budgets = append(b.sheet.Budgets, *budget)
	}
	slices.SortFunc(b.sheet.Budgets, func(a, b BudgetYear) int { return cmp.Compare(a.Year, b.Year) })
	return nil
}

func checkBudgetTotals(budget *BudgetYear) {
	checks := []struct {
		name   string
		total  Yen
		values func(BudgetAccount) []Yen
	}{
		{"当初予算", budget.Total.Initial, func(a BudgetAccount) []Yen { return []Yen{a.Initial} }},
		{"補正予算", budget.Total.Supplementary, func(a BudgetAccount) []Yen { return a.Supplementary[:] }},
		{"前年度からの繰越し", budget.Total.CarriedIn, func(a BudgetAccount) []Yen { return []Yen{a.CarriedIn} }},
		{"予備費等", budget.Total.Reserve, func(a BudgetAccount) []Yen { return a.Reserve[:] }},
		{"歳出予算現額", budget.Total.Current, func(a BudgetAccount) []Yen { return []Yen{a.Current} }},
		{"執行額", budget.Total.Executed, func(a BudgetAccount) []Yen { return []Yen{a.Executed} }},
		{"翌年度要求額", budget.Total.NextRequest, func(a BudgetAccount) []Yen { return []Yen{a.NextRequest} }},
	}
	for _, check := range checks {
		if !check.total.Valid {
			continue // A blank published value cannot be compared as zero.
		}
		// Keep the comparison exact even when the sum exceeds int64.
		var sum big.Int
		for _, account := range budget.Accounts {
			for _, value := range check.values(account) {
				if value.Valid {
					sum.Add(&sum, big.NewInt(value.Value))
				}
			}
		}
		if sum.Cmp(big.NewInt(check.total.Value)) != 0 {
			budget.Issues = append(budget.Issues, fmt.Sprintf(
				"%sの合計行（%d）と会計別行の合計（%s）が不一致です", check.name, check.total.Value, &sum))
		}
	}
}

var itemColumns = []string{
	"予算年度", "会計区分", "会計", "勘定", "予算種別", "所管", "組織・勘定", "項", "目",
	"歳出予算項目の補足情報", "予算額（歳出予算項目ごと）", "翌年度要求額（歳出予算項目ごと）",
}

type itemBuilder struct{ sheet *Sheet }

func (b *itemBuilder) row(r *csvRow) error {
	b.sheet.Items = append(b.sheet.Items, BudgetItem{
		Year: r.year("予算年度"), Category: r.text("会計区分"), Account: r.text("会計"),
		Subaccount: r.text("勘定"), Kind: r.text("予算種別"), Owner: r.text("所管"),
		Org: r.text("組織・勘定"), Ko: r.text("項"), Moku: r.text("目"),
		Note: r.text("歳出予算項目の補足情報"), Amount: r.yen("予算額（歳出予算項目ごと）"),
		NextRequest: r.yen("翌年度要求額（歳出予算項目ごと）"),
	})
	return nil
}

func (b *itemBuilder) finish() error { return nil }
