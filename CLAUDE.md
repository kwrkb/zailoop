# zailoop

予算事業レビューシート CSV から、1 事業の予算ライフサイクルを可視化する Go CLI。

## ドキュメント

矛盾時の優先順: VISION.md > PLAN.md > コード > NOTES.md。LESSONS.md は記録。
データの列対応は `docs/data-survey.md` が正。

## 開発

- Go stdlib のみ。外部依存を足すときは VISION.md に理由を書く
- `go build ./... && go vet ./... && go test ./...` を通してから報告する
- `data/` はコミットしない。テストは `testdata/` の小さな抜粋 CSV を使う
- 実データでの確認は `data/csv/` を読む（`zailoop fetch --year 2024` で取得）
- サイトへのアクセスは取得スクリプト経由のみ。テストからネットワークに出ない

## 表記

- 出力には出典「行政事業レビュー見える化サイトのデータを加工して作成」を必ず含める
- 「RSシステム」「行政事業レビュー」を製品名のように使わない
