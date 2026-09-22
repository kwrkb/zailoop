# NOTES（内部構造の索引。鮮度は保証しない）

## パッケージ

| パス | 役割 | I/O |
|---|---|---|
| `cmd/zailoop` | サブコマンド `fetch` / `show` | あり |
| `internal/fetch` | 配布 ZIP の取得（1 ファイル 1 回）、検証、展開 | HTTP / ファイル |
| `internal/rs` | CSV 読み取りと `Sheet` の組み立て。列名を知る唯一の場所 | ファイル |
| `internal/lifecycle` | `Sheet` → 6 段階 `Lifecycle`。年度の対応、欠測・算出不能の状態判定 | なし |
| `internal/render` | `Lifecycle` → テキスト。出典表記と金額整形関数（site からも使う） | `io.Writer` |
| `internal/site` | 全事業を静的サイトに書き出す。`Build(src, out, opt)`。テンプレートとアセットは `embed` | ファイル |
| `web/` | index.html 用フロント（TypeScript）。`npm run build` で `internal/site/assets/app.js` を更新 | ブラウザ |

## site の出力

- `site/index.html` 一覧・断絶・成果の 3 ビュー。データは `<script id="zailoop-data" type="application/json">` に埋め込み（`template.JS` で素通し）
- `site/list.html` JS なしの府省庁別リンク一覧
- `site/p/<ID>.html` 事業詳細（JS なし、相対パスで `../assets/`）
- 一覧 JSON のキーは `internal/site/index.go` の `Row`、デコードは `web/src/data.ts`

## 全事業の読み取り（rs.Each）

6 ファイルを `groupReader` で同時に開き、各ファイルの「同一 ID の連続行」を ID 昇順にマージして 1 事業ずつ `Sheet` を組み立てる。ID が昇順でなければ `ErrUnordered`。`LoadSheet` も同じ `builder`/`table` を使う。

## データの置き場

- `data/raw/<name>.zip` 取得した ZIP（不正なものは `.bad` に退避）
- `data/csv/<name>.csv` 展開した CSV
- `testdata/2024/` 4 事業分の抜粋（ID 11, 884, 3522, 18556）

## 年度の対応（lifecycle）

事業年度 S のシートで N = S − 1 を軸にする。① 要求は予算年度 N−1 行の翌年度要求額、②③④ は予算年度 N 行、⑥ の次年度要求は予算年度 N+1 行の翌年度要求額（= FY N+2）。

## 参照

- 列の仕様と品質問題: `docs/data-survey.md`
