package rs

import (
	"fmt"
	"strings"
)

var payeeColumns = []string{
	"支出先ブロック番号", "支出先ブロック名", "支出先の数", "事業を行う上での役割", "ブロックの合計支出額",
	"支出先名", "法人番号", "所在地", "法人種別", "支出先の合計支出額",
	"契約概要", "金額", "契約方式等", "具体的な契約方式等", "入札者数", "落札率",
}

type payeeBuilder struct {
	sheet                  *Sheet
	blockIndex, payeeIndex int
}

// row は 5-1 の 1 行を読む。
//
// 2025 年度シートまでは、ブロック行（ブロックの合計支出額）・支出先行（支出先の合計支出額）・
// 契約行（金額）が別の行に並ぶ。2026 年度シートは 1 行が 1 契約で、ブロックの最初の行に
// ブロックの合計、支出先の最初の行に支出先の合計が同じ行に載る。どちらも上位から順に読む。
func (b *payeeBuilder) row(r *csvRow) error {
	blockTotal := r.yen("ブロックの合計支出額")
	payeeTotal := r.yen("支出先の合計支出額")
	amount := r.yen("金額")
	if r.err != nil {
		return r.err
	}
	if !blockTotal.Valid && !payeeTotal.Valid && !amount.Valid {
		return nil // Projects without expenditure have an empty placeholder row.
	}
	if blockTotal.Valid && amount.Valid && !payeeTotal.Valid {
		return fmt.Errorf("ブロックの合計と契約の金額がある行に支出先の合計がありません")
	}
	number := strings.TrimSpace(r.text("支出先ブロック番号"))
	if blockTotal.Valid {
		b.sheet.Blocks = append(b.sheet.Blocks, PayeeBlock{
			Number: number, Name: r.text("支出先ブロック名"), PayeeCount: r.text("支出先の数"),
			Role: r.text("事業を行う上での役割"), Total: blockTotal,
		})
		b.blockIndex, b.payeeIndex = len(b.sheet.Blocks)-1, -1
	}
	if !payeeTotal.Valid && !amount.Valid {
		return nil
	}
	if b.blockIndex < 0 {
		return fmt.Errorf("支出先・契約行の前にブロック行がありません")
	}
	block := &b.sheet.Blocks[b.blockIndex]
	if number != "" && number != block.Number {
		return fmt.Errorf("支出先ブロック番号 %q が直前のブロック %q と一致しません", number, block.Number)
	}
	if payeeTotal.Valid {
		block.Payees = append(block.Payees, Payee{
			Name: r.text("支出先名"), CorporateNumber: strings.TrimSpace(r.text("法人番号")),
			Location: r.text("所在地"), Kind: r.text("法人種別"), Total: payeeTotal,
		})
		b.payeeIndex = len(block.Payees) - 1
	}
	if !amount.Valid {
		return nil
	}
	if b.payeeIndex < 0 {
		return fmt.Errorf("契約行の前に支出先合計行がありません")
	}
	payee := &block.Payees[b.payeeIndex]
	payee.Contracts = append(payee.Contracts, Contract{
		Summary: r.text("契約概要"), Amount: amount, Method: r.text("契約方式等"),
		MethodDetail: r.text("具体的な契約方式等"), Bidders: r.text("入札者数"), Rate: r.text("落札率"),
	})
	return nil
}

func (b *payeeBuilder) finish() error { return nil }
