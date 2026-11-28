package rs

import (
	"cmp"
	"fmt"
	"math/big"
	"slices"
	"strings"
)

// carriedOutColumn は 2025 年度シートまでの 2-1 にある列。2026 年度シートで無くなった。
// この列がないファイルを 2026 年度形式として読む（budgetBuilder.merged）。
const carriedOutColumn = "翌年度への繰越し(合計）"

var totalColumns = []string{
	"当初予算（合計）", "補正予算（合計）", "前年度からの繰越し（合計）", "予備費等（合計）",
	"計（歳出予算現額合計）", "執行額（合計）", "執行率", "翌年度要求額（合計）",
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

// budgetBuilder は 2-1 を予算年度ごとにまとめる。
//
// 2025 年度シートまでは、合計行（合計列だけが埋まる）と会計別行（会計別列だけが埋まる）が別の行で、
// 金額の 0 は明示的に 0 と書かれる。2026 年度シートは次の点が違う（merged）。
//   - 合計列が会計別行の各行に繰り返し載る。同じ値の繰り返しは 1 つの合計として扱う
//   - 金額の 0 が空欄で書かれる。その予算年度の行がある場合に限り、金額の空欄を 0 と読む
//     （すべて 0 の年度は、会計区分だけが埋まった行になる）
//   - 翌年度への繰越しの列がない。翌予算年度の「前年度からの繰越し」を FY N の翌年度繰越とする
//     （2025 年度シートでは両者が全組で一致した。docs/data-survey.md）
//   - 執行率が百分率（79.6）で書かれる。2025 年度シートまでと同じ比率（0.796）に直す
//   - 予算のない事業は予算年度も空欄の 1 行だけを持つ
type budgetBuilder struct {
	sheet       *Sheet
	byYear      map[int]*BudgetYear
	totalCounts map[int]int
	merged      bool
}

func (b *budgetBuilder) row(r *csvRow) error {
	hasTotal := r.hasAny(totalColumns) || strings.TrimSpace(r.text(carriedOutColumn)) != ""
	hasAccount := r.hasAny(accountColumns)
	if !hasTotal && !hasAccount && strings.TrimSpace(r.text("予算年度")) == "" {
		return nil // Projects without any budget year have an empty placeholder row.
	}
	b.merged = !r.has(carriedOutColumn)
	if b.merged {
		hasTotal, hasAccount = true, true // すべて 0 の年度は金額がすべて空欄の行になる
	}
	year := r.year("予算年度")
	budget := b.byYear[year]
	if budget == nil {
		budget = &BudgetYear{Year: year}
		b.byYear[year] = budget
	}
	if !hasTotal && !hasAccount {
		budget.Issues = append(budget.Issues, "合計列・会計別列がともに空の行を無視しました")
		return nil
	}
	yen := r.yen
	if b.merged {
		yen = func(name string) Yen {
			if v := r.yen(name); v.Valid {
				return v
			}
			return Yen{Valid: true}
		}
	}
	if hasTotal {
		total := BudgetTotal{
			Initial: yen("当初予算（合計）"), Supplementary: yen("補正予算（合計）"),
			CarriedIn: yen("前年度からの繰越し（合計）"), Reserve: yen("予備費等（合計）"),
			Current: yen("計（歳出予算現額合計）"), Executed: yen("執行額（合計）"),
			ExecRate: r.ratio("執行率"), NextRequest: yen("翌年度要求額（合計）"),
			ChangeReason: r.text("主な増減理由"), Notes: r.text("その他特記事項"),
		}
		if b.merged {
			total.ExecRate.Value /= 100
			blank := func(name string) bool { return strings.TrimSpace(r.text(name)) == "" }
			total.Blank = BudgetBlank{Initial: blank("当初予算（合計）"), Current: blank("計（歳出予算現額合計）"), Executed: blank("執行額（合計）")}
		} else {
			total.CarriedOut = r.yen(carriedOutColumn)
		}
		// 空欄と 0 は同じ値として比べる。空欄の印は、繰り返しのすべての行で空欄だった項目にだけ残す
		// （どこかの行に 0 と書かれていれば、府省が 0 と書いたものとする）。
		blank := total.Blank
		total.Blank = budget.Total.Blank
		switch {
		case b.totalCounts[year] == 0:
			total.Blank = blank
			budget.Total = total
			b.totalCounts[year]++
		case !b.merged || total != budget.Total:
			b.totalCounts[year]++
		default:
			budget.Total.Blank.and(blank)
		}
	}
	if hasAccount {
		account := BudgetAccount{
			Category: r.text("会計区分"), Account: r.text("会計"), Subaccount: r.text("勘定"),
			Initial: yen("当初予算"), CarriedIn: yen("前年度から繰越し"),
			Current: yen("歳出予算現額"), Executed: yen("執行額"),
			NextRequest: yen("翌年度要求額"), Demand: yen("要望額"), Notes: r.text("備考"),
		}
		for i := range account.Supplementary {
			account.Supplementary[i] = yen(fmt.Sprintf("第%d次補正予算", i+1))
		}
		for i := range account.Reserve {
			account.Reserve[i] = yen(fmt.Sprintf("予備費等%d", i+1))
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
		case count > 1 && b.merged:
			budget.Issues = append(budget.Issues, fmt.Sprintf("合計列の値が行によって異なります（%d通り）。最初の行を保持しました", count))
		case count > 1:
			budget.Issues = append(budget.Issues, fmt.Sprintf("合計行が重複しています（%d行）。最初の行を保持しました", count))
		}
		if b.merged && count > 0 {
			budget.Total.CarriedOutDerived = true
			// 翌予算年度の合計が 1 通りに決まらないときは、繰越も分からないものとする。
			if next := b.byYear[year+1]; next != nil && b.totalCounts[year+1] == 1 {
				budget.Total.CarriedOut = next.Total.CarriedIn
			}
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
	if !r.hasAny(itemColumns) {
		return nil // Projects without any budget year have an empty placeholder row (2026 年度シート).
	}
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
