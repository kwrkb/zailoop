package rs

import (
	"fmt"
	"strings"
)

func (d Dir) loadPayees(id string, sheet *Sheet) error {
	columns := []string{
		"支出先ブロック番号", "支出先ブロック名", "支出先の数", "事業を行う上での役割", "ブロックの合計支出額",
		"支出先名", "法人番号", "所在地", "法人種別", "支出先の合計支出額",
		"契約概要", "金額", "契約方式等", "具体的な契約方式等", "入札者数", "落札率",
	}
	blockIndex, payeeIndex := -1, -1
	return d.scan("5-1", id, columns, func(r *csvRow) error {
		blockTotal := r.yen("ブロックの合計支出額")
		payeeTotal := r.yen("支出先の合計支出額")
		amount := r.yen("金額")
		if r.err != nil {
			return r.err
		}
		if !blockTotal.Valid && !payeeTotal.Valid && !amount.Valid {
			return nil // Projects without expenditure have an empty placeholder row.
		}
		if (blockTotal.Valid && (payeeTotal.Valid || amount.Valid)) || (payeeTotal.Valid && amount.Valid) {
			return fmt.Errorf("ブロック・支出先・契約の金額が同じ行に混在しています")
		}
		number := strings.TrimSpace(r.text("支出先ブロック番号"))
		if blockTotal.Valid {
			sheet.Blocks = append(sheet.Blocks, PayeeBlock{
				Number: number, Name: r.text("支出先ブロック名"), PayeeCount: r.text("支出先の数"),
				Role: r.text("事業を行う上での役割"), Total: blockTotal,
			})
			blockIndex, payeeIndex = len(sheet.Blocks)-1, -1
			return nil
		}
		if blockIndex < 0 {
			return fmt.Errorf("支出先・契約行の前にブロック行がありません")
		}
		block := &sheet.Blocks[blockIndex]
		if number != "" && number != block.Number {
			return fmt.Errorf("支出先ブロック番号 %q が直前のブロック %q と一致しません", number, block.Number)
		}
		if payeeTotal.Valid {
			block.Payees = append(block.Payees, Payee{
				Name: r.text("支出先名"), CorporateNumber: strings.TrimSpace(r.text("法人番号")),
				Location: r.text("所在地"), Kind: r.text("法人種別"), Total: payeeTotal,
			})
			payeeIndex = len(block.Payees) - 1
			return nil
		}
		if payeeIndex < 0 {
			return fmt.Errorf("契約行の前に支出先合計行がありません")
		}
		payee := &block.Payees[payeeIndex]
		payee.Contracts = append(payee.Contracts, Contract{
			Summary: r.text("契約概要"), Amount: amount, Method: r.text("契約方式等"),
			MethodDetail: r.text("具体的な契約方式等"), Bidders: r.text("入札者数"), Rate: r.text("落札率"),
		})
		return nil
	})
}
