package rs

// 5-4 の列名には「（国庫債務負担行為等による契約）」が付く。
const obligationSuffix = "（国庫債務負担行為等による契約）"

var obligationColumns = []string{
	"支出先ブロック" + obligationSuffix, "契約先名" + obligationSuffix, "契約概要（契約名）" + obligationSuffix,
	"契約額" + obligationSuffix, "契約方式等" + obligationSuffix, "入札者数（応募者数）" + obligationSuffix,
	"落札率（％）" + obligationSuffix,
}

type obligationBuilder struct{ sheet *Sheet }

func (b *obligationBuilder) row(r *csvRow) error {
	// 契約のない事業は空欄 1 行だけを持つ。契約先だけ・契約額だけの行もあるので、どれかが埋まっていれば残す。
	if !r.hasAny(obligationColumns) {
		return nil
	}
	amount := r.yen("契約額" + obligationSuffix)
	if r.err != nil {
		return r.err
	}
	b.sheet.Obligations = append(b.sheet.Obligations, Obligation{
		Block: r.text("支出先ブロック" + obligationSuffix), Payee: r.text("契約先名" + obligationSuffix),
		Summary: r.text("契約概要（契約名）" + obligationSuffix), Amount: amount,
		Method: r.text("契約方式等" + obligationSuffix), Bidders: r.text("入札者数（応募者数）" + obligationSuffix),
		Rate: r.text("落札率（％）" + obligationSuffix),
	})
	return nil
}

func (b *obligationBuilder) finish() error { return nil }
