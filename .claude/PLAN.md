# PLAN

現在地: **Phase 3a 完了。次は Phase 3b（2026 年度シート公開後。再開手順は `.claude/handoff-phase3b.md`）**

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

## Phase 3a: 2024・2025 年度の結合とループ検証（完了）

完了条件:
- [x] `rs.Multi.Each` で複数年度を予算事業ID でマージして読める（実データ 6,227 事業）
- [x] `lifecycle.Track` が年度をまたぐ推移（新しいシートを正、差異は注記）とループ検証（Verdict）を作る
- [x] `show --years 2024,2025` に推移とループ検証、`build --years` の詳細ページと index.html の 4 ビュー目「ループ検証」
- [x] ループ用 Signal（反映と逆行、要求ゼロ査定）
- [x] 実データで Verdict の分布が事前集計と一致（縮減 148 減 / 45 増、廃止 14 減、反映要確認（旧称: 矛盾）47 件）
- [x] `go test ./...` と `npm run verify` が通る

## Phase 3b: 3 年以上の追跡と ID 変更の追跡（未着手、2026 年度公開後）

完了条件:

- [ ] `--years 2024,2025,2026` で 3 年連続の推移とループ検証
- [ ] 2026 年度の CSV 構成（ヘッダ）が同じか確認する仕組み
- [ ] ID が変わった継続事業の候補提示（名称・所管・関連事業の一致。自動結合はしない）
