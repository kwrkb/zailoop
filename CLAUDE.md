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
- `web/src` を変えたら `cd web && npm run verify` で `internal/site/assets/app.js` を再生成し、`git diff --exit-code internal/site/assets/app.js` が空でないことを確認してからコミットする（成果物はコミット対象）
- 断絶判定（Signal）とループ検証（Verdict）は `internal/lifecycle` だけで行う。フロントで再判定しない
- 複数年度の結合は予算事業ID のみ。重なる年度は新しいシートを正にする（VISION 11）

## 表記

- 出力には出典 `render.Attribution`（「行政事業レビュー見える化サイトのデータを加工して作成」＋加工の主体・データの URL・PDL1.0）と注記 `render.Disclaimer`（非公式・独自の判定）を必ず含める。HTML は `templates/partials.html` の `credit` を使う
- 兆候・ループ検証を公式の評価のように見せない（見える化サイトの規約: 加工した情報を国が作成したかのような態様で公表しない）
- 「RSシステム」「行政事業レビュー」を製品名のように使わない
