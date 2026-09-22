package rs

import "slices"

func (d Dir) loadEvaluation(id string, sheet *Sheet) error {
	columns := []string{
		"事業所管部局による点検・改善ー点検結果", "事業所管部局による点検・改善ー改善の方向性",
		"事業所管部局による点検・改善－目標年度における効果測定に関する評価",
		"外部有識者による点検ー最終実施年度", "外部有識者による点検ー点検対象",
		"外部有識者による点検ー対象の理由", "外部有識者による点検ー所見", "公開プロセス結果概要",
		"行政事業レビュー推進チームの所見", "行政事業レビュー推進チームの所見の詳細",
		"所見を踏まえた改善点／概算要求における反映状況", "所見を踏まえた改善点／概算要求における反映状況の詳細",
		"反映額（一般会計）", "反映額（特別会計）－会計", "反映額（特別会計）－勘定", "反映額（特別会計）－反映額",
		"過去に受けた指摘事項－区分", "過去に受けた指摘事項－取りまとめ年度",
		"過去に受けた指摘事項－取りまとめ内容", "過去に受けた指摘事項－対応状況",
		"その他の指摘事項－調査等の名称", "その他の指摘事項－指摘年度",
		"その他の指摘事項－指摘内容", "その他の指摘事項－対応状況",
	}
	found := false
	return d.scan("4-1", id, columns, func(r *csvRow) error {
		e := &sheet.Evaluation
		general := r.yen("反映額（一般会計）")
		if !found {
			*e = Evaluation{
				SelfCheck:            r.text("事業所管部局による点検・改善ー点検結果"),
				Improvement:          r.text("事業所管部局による点検・改善ー改善の方向性"),
				TargetYearAssessment: r.text("事業所管部局による点検・改善－目標年度における効果測定に関する評価"),
				ExternalYear:         r.text("外部有識者による点検ー最終実施年度"),
				ExternalTarget:       r.text("外部有識者による点検ー点検対象"),
				ExternalReason:       r.text("外部有識者による点検ー対象の理由"),
				ExternalOpinion:      r.text("外部有識者による点検ー所見"),
				PublicProcess:        r.text("公開プロセス結果概要"),
				TeamOpinion:          r.text("行政事業レビュー推進チームの所見"),
				TeamOpinionDetail:    r.text("行政事業レビュー推進チームの所見の詳細"),
				Reflection:           r.text("所見を踏まえた改善点／概算要求における反映状況"),
				ReflectionDetail:     r.text("所見を踏まえた改善点／概算要求における反映状況の詳細"),
				ReflectedGeneral:     general,
			}
			found = true
		}
		special := SpecialReflection{
			Account: r.text("反映額（特別会計）－会計"), Subaccount: r.text("反映額（特別会計）－勘定"),
			Amount: r.yen("反映額（特別会計）－反映額"),
		}
		if special != (SpecialReflection{}) && !slices.Contains(e.ReflectedSpecial, special) {
			e.ReflectedSpecial = append(e.ReflectedSpecial, special)
		}
		past := Remark{
			Kind: r.text("過去に受けた指摘事項－区分"), Year: r.text("過去に受けた指摘事項－取りまとめ年度"),
			Content: r.text("過去に受けた指摘事項－取りまとめ内容"), Status: r.text("過去に受けた指摘事項－対応状況"),
		}
		if past != (Remark{}) && !slices.Contains(e.PastRemarks, past) {
			e.PastRemarks = append(e.PastRemarks, past)
		}
		other := Remark{
			Kind: r.text("その他の指摘事項－調査等の名称"), Year: r.text("その他の指摘事項－指摘年度"),
			Content: r.text("その他の指摘事項－指摘内容"), Status: r.text("その他の指摘事項－対応状況"),
		}
		if other != (Remark{}) && !slices.Contains(e.OtherRemarks, other) {
			e.OtherRemarks = append(e.OtherRemarks, other)
		}
		return nil
	})
}
