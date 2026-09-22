# PLAN

現在地: **Phase 2 完了。次は Phase 3（複数年度の結合と追跡）**

## Phase 1a: データ調査（完了）

- [x] 2024 年度 CSV を取得・展開
- [x] `docs/data-survey.md`（ファイル一覧、キー、ライフサイクル対応表、品質問題、表示方式の提案）
- [x] ユーザー確認

## Phase 1b: 単一事業ライフサイクル表示（完了）

完了条件:
- [x] `zailoop fetch --year 2024` で 15 ファイルを取得・展開できる
- [x] CSV パーサ（BOM 除去、TrimSpace）と事業単位の構造体
- [x] `zailoop show <予算事業ID>` で要求〜翌年度反映が矛盾なく表示される
- [x] `go build ./...` と `go test ./...` が通る
- [x] 出典表記がある
- [x] VISION.md / PLAN.md / CLAUDE.md の初版

## Phase 2: 静的 HTML と横断ダッシュボード（完了）

完了条件:
- [x] `rs.Dir.Each` で全事業を 1 パスで読める（実データ 5,664 件を約 2 秒）
- [x] `lifecycle` に断絶判定（Signal）と一覧サマリ（Summary）がある。閾値は `build` の引数
- [x] `zailoop build --out site` で index.html / list.html / p/<ID>.html × 5,664 / assets を生成（約 5 秒、68MB、index.html 1.5MB）
- [x] 一覧・断絶・成果の 3 ビューが TypeScript で動く（file:// と http:// の両方をヘッドレス Chrome で確認）
- [x] `go test ./...` と `npm run verify`（型検査・テスト・バンドル）が通る
- [x] 出典表記が index / list / 詳細のすべてにある

## Phase 3: 複数年度の結合と追跡（未着手）
