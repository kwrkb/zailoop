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
- 当初予算が 0 で補正予算がある事業は、JSON の `sp`（補正、0 以外のときだけ）を一覧の当初セルに「補正 ○円」と添え、詳細の要点にも 1 文足す

## 複数年度（rs.Multi / lifecycle.Track）

- `rs.Multi{Dirs}` は各年度の `Dir` のイテレータを並走させ、同じ ID のシートを年度順に `[]*Sheet` で渡す。`Dir.Year` と CSV の事業年度が違えばエラー
- `lifecycle.Track(sheets)` → `Timeline{Years, Loops, Names, Notes}`。`Years` は予算年度ごとに最新シートの値、`Loops` はシート S の反映 → S+1 の当初・執行
- `YearRow.Revised` は古いシートとの差異（`Revision`、`String()` が注記の文言）。兆候「シート間の改訂」に数えるのは `QualifyingRevisions` で絞ったものだけ（注記はすべての差異）
- `SummarizeTimeline` は最新シートの `Summary` に `PrevReflection / PrevInitial / LoopVerdict / Renamed` を足す。JSON キーは `pr, pi, lv, rn`
- フロントの「ループ検証」ビュー（`web/src/views/loops.ts`）は `nextInitial − prevInitial` で増減を出す（判定自体は Go の Verdict）

## 全事業の読み取り（rs.Each）

8 ファイル（1-2, 1-5, 2-1, 2-2, 3-1, 4-1, 5-1, 5-4）を `groupReader` で同時に開き、各ファイルの「同一 ID の連続行」を ID 昇順にマージして 1 事業ずつ `Sheet` を組み立てる。ID が昇順でなければ `ErrUnordered`。`LoadSheet` も同じ `builder`/`table` を使う。

## 関連事業（1-5）

- `rs.Sheet.Related` → `lifecycle.Lifecycle.Related`。`Parents()` は関連性が「親事業」のもの、`AllZero` は全予算年度の金額がすべて 0
- `lifecycle.Timeline.PastParents` は古いシートにだけある親事業（年度つき）
- `site.BuildMulti` は先に `rs.Multi.IDs()`（1-2 だけ読む）でページのある ID を集め、関連事業はその ID だけリンクにする
- 詳細ページ: `AllZero` なら要点の下に「金額がすべて 0 の事業」（全年度の特記事項・増減理由を重複除去、基本情報の備考、5-4 の契約があればその旨）、各段階の後に「国庫債務負担行為等による契約」「関連事業」、② 成立に FY N の「その他特記事項」
- `rs.Project.Remarks`（1-2 備考）・`URL`（事業概要URL）は「事業の目的」などと同じ欄に出す（`AllZero` の事業は備考を上の注記に出す）。`rs.Sheet.Obligations`（5-4）→ `lifecycle.Lifecycle.Obligations`。金額の集計・判定には使わない

## データの置き場

- `data/raw/<name>.zip` 取得した ZIP（不正なものは `.bad` に退避）
- `data/csv/<name>.csv` 展開した CSV
- `testdata/2024/` 5 事業分の抜粋（ID 11, 884, 1319, 3522, 18556）、`testdata/2025/` 同 7 事業（+1937〔親事業 1936 の子、金額がすべて 0〕, 21625）

## 年度の対応（lifecycle）

事業年度 S のシートで N = S − 1 を軸にする。① 要求は予算年度 N−1 行の翌年度要求額、②③④ は予算年度 N 行、⑥ の次年度要求は予算年度 N+1 行の翌年度要求額（= FY N+2）。

## 参照

- 列の仕様と品質問題: `docs/data-survey.md`
