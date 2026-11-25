# NOTES（内部構造の索引。鮮度は保証しない）

## パッケージ

| パス | 役割 | I/O |
|---|---|---|
| `cmd/zailoop` | サブコマンド `fetch` / `show` / `build` | あり |
| `internal/fetch` | 配布 ZIP の取得（1 ファイル 1 回）、検証、展開 | HTTP / ファイル |
| `internal/rs` | CSV 読み取りと `Sheet` の組み立て。列名を知る唯一の場所。`schema.go` はヘッダの年度間比較（`CompareHeaders`）と必須列の不足（`MissingColumns`）で、`fetch` の最後に `cmd/zailoop` の `checkHeaders` が出す | ファイル |
| `internal/lifecycle` | `Sheet` → 6 段階 `Lifecycle`。年度の対応、欠測・算出不能の状態判定 | なし |
| `internal/render` | `Lifecycle` → テキスト。出典表記と金額整形関数（site からも使う） | `io.Writer` |
| `internal/site` | 全事業を静的サイトに書き出す。`Build(src, out, opt)`。テンプレートとアセットは `embed` | ファイル |
| `web/` | index.html 用フロント（TypeScript）。`npm run build` で `internal/site/assets/app.js` を更新 | ブラウザ |

## site の出力

- `site/index.html` 一覧・断絶・成果・ループ検証の 4 ビュー。データは `<script id="zailoop-data" type="application/json">` に埋め込み（`template.JS` で素通し）
- `site/list.html` JS なしの府省庁別リンク一覧
- `site/p/<ID>.html` 事業詳細（JS なし、相対パスで `../assets/`）
- `site/terms.html` 用語の説明（`internal/site/terms.go`）
- `site/` は出力先を消さずに上書きする（`_redirects` を残すため）。公開手順は README と `wrangler.jsonc` のコメント
- 一覧 JSON のキーは `internal/site/index.go` の `Row`、デコードは `web/src/data.ts`
- 当初予算が 0 で補正予算がある事業は、JSON の `sp`（補正、0 以外のときだけ）を一覧の当初セルに「補正 ○円」と添え、詳細の要点にも 1 文足す

## 複数年度（rs.Multi / lifecycle.Track）

- `rs.Multi{Dirs}` は各年度の `Dir` のイテレータを並走させ、同じ ID のシートを年度順に `[]*Sheet` で渡す。`Dir.Year` と CSV の事業年度が違えばエラー
- `lifecycle.Track(sheets)` → `Timeline{Years, Loops, Names, Notes}`。`Years` は予算年度ごとに最新シートの値、`Loops` はシート S の反映 → S+1 の当初・執行
- `YearRow.Revised` は古いシートとの差異（`Revision`、`String()` が注記の文言）。兆候「シート間の改訂」に数えるのは `QualifyingRevisions` で絞ったものだけ（注記はすべての差異）
- `SummarizeTimeline` は最新シートの `Summary` に `PrevReflection / PrevInitial / LoopVerdict / Renamed` を足す。JSON キーは `pr, pi, lv, rn`
- フロントの「ループ検証」ビュー（`web/src/views/loops.ts`）は `nextInitial − prevInitial` で増減を出す（判定自体は Go の Verdict）

## 全事業の読み取り（rs.Each）

8 ファイル（1-2, 1-5, 2-1, 2-2, 3-1, 4-1, 5-1, 5-4）を `groupReader` で同時に開き、各ファイルの「同一 ID の連続行」を ID 昇順にマージして 1 事業ずつ `Sheet` を組み立てる。ID が昇順でなければ `ErrUnordered`。`LoadSheet` も同じ `builder`/`table` を使う。

## データ出典（rs.Source）

- `rs.Sheet.Source{Year, Files[]{Table, Name}}` は読んだ時点で記録する（`LoadSheet` は `scan` の戻り値、`Each` は `sheetIter.source` を Sheet ごとに複製）。ファイル名だけでディレクトリは持たない
- `lifecycle.Lifecycle.Source`、`lifecycle.Timeline.Sources`（事業年度の昇順、手元にあるシートの分だけ）
- 要約の文言は `render.SourceSummary`（CLI の出典表記の下と、詳細ページのフッター `<details class="data-source">` で共用）。`show --sources` は `render.Options.SourceFiles` で要約の下にファイル名を並べる
- `render.Timeline` は本文（`text`）→ 推移・ループ検証 → `footer`（出典表記・要約・ファイル名・注記）の順。出典表記は末尾に 1 回

## 関連事業（1-5）

- `rs.Sheet.Related` → `lifecycle.Lifecycle.Related`。`Parents()` は関連性が「親事業」のもの、`AllZero` は全予算年度の金額がすべて 0
- `lifecycle.Timeline.PastParents` は古いシートにだけある親事業（年度つき）
- `site.BuildMulti` は先に `rs.Multi.IDs()`（1-2 だけ読む）でページのある ID を集め、関連事業はその ID だけリンクにする
- 詳細ページ: `AllZero` なら要点の下に「金額がすべて 0 の事業」（全年度の特記事項・増減理由を重複除去、基本情報の備考、5-4 の契約があればその旨）、各段階の後に「国庫債務負担行為等による契約」「関連事業」、② 成立に FY N の「その他特記事項」
- `rs.Project.Remarks`（1-2 備考）・`URL`（事業概要URL）は「事業の目的」などと同じ欄に出す（`AllZero` の事業は備考を上の注記に出す）。`rs.Sheet.Obligations`（5-4）→ `lifecycle.Lifecycle.Obligations`。金額の集計・判定には使わない

## データの置き場

- `data/raw/<name>.zip` 取得した ZIP（不正なものは `.bad` に退避）
- `data/raw/fetch-<year>.json` 取得記録（ZIP・CSV の SHA-256、`fetched_at`、`checked_at`）。`internal/fetch/manifest.go`
- `data/raw/old/<name>.<SHA-256 先頭 12 桁>.zip` `fetch --refresh` で差し替えた旧版
- `data/csv/<name>.csv` 展開した CSV
- `testdata/2024/` 5 事業分の抜粋（ID 11, 884, 1319, 3522, 18556）、`testdata/2025/` 同 7 事業（+1937〔親事業 1936 の子、金額がすべて 0〕, 21625）、`testdata/2026/` 同 7 事業 + 21437（予算のない事業）。用途は `testdata/README.md`

## 年度による形式の違い（rs）

- 列名の新旧は `internal/rs/csv.go` の `columnAliases`（キーは 2025 年度までの列名）。ヘッダにキーの列がなければ別名を探す
- 2-1 の読み方は「`翌年度への繰越し(合計）` 列（`carriedOutColumn`）があるか」で切り替える（`internal/rs/budget.go`）。ない形式（2026〜）では、行がある予算年度の空欄を 0、執行率を 100 で割る、翌年度繰越は翌予算年度の前年度から繰越し
- 5-1 は新旧どちらの行構造も上位（ブロック → 支出先 → 契約）から順に読む（`internal/rs/payee.go`）
- 4-1 の `予定通り終了` はループ検証で `終了予定` と同じに扱う
- 2026 形式のテストは `internal/rs/sheet2026_test.go`。変更点の一覧は `docs/data-survey.md` §9

## 年度の対応（lifecycle）

事業年度 S のシートで N = S − 1 を軸にする。① 要求は予算年度 N−1 行の翌年度要求額、②③④ は予算年度 N 行、⑥ の次年度要求は予算年度 N+1 行の翌年度要求額（= FY N+2）。

## 開発用の補助（`.claude/`、`//go:build ignore`）

- `remarks-match.go` 備考の過年度執行額と 5-1 の照合（確認記録の `sheet_ref` 候補）
- `id-candidates.go` ID が変わった継続事業の前身の候補（`旧事業番号`・1-5・名称）。サイトには出さない
- `loop-items.go` 反映要確認のループを 2-2 の目ごとに分ける（当初・要求・翌年度の当初とレコード番号、2-1 との照合、反映額と同額の減）。確認記録の `sheet_ref` 候補

## 参照

- 列の仕様と品質問題: `docs/data-survey.md`
