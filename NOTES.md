# NOTES（内部構造の索引。鮮度は保証しない）

## パッケージ

| パス | 役割 | I/O |
|---|---|---|
| `cmd/zailoop` | サブコマンド `fetch` / `show` | あり |
| `internal/fetch` | 配布 ZIP の取得（1 ファイル 1 回）、検証、展開 | HTTP / ファイル |
| `internal/rs` | CSV 読み取りと `Sheet` の組み立て。列名を知る唯一の場所 | ファイル |
| `internal/lifecycle` | `Sheet` → 6 段階 `Lifecycle`。年度の対応、欠測・算出不能の状態判定 | なし |
| `internal/render` | `Lifecycle` → テキスト。出典表記を含む | `io.Writer` |

## データの置き場

- `data/raw/<name>.zip` 取得した ZIP（不正なものは `.bad` に退避）
- `data/csv/<name>.csv` 展開した CSV
- `testdata/2024/` 4 事業分の抜粋（ID 11, 884, 3522, 18556）

## 年度の対応（lifecycle）

事業年度 S のシートで N = S − 1 を軸にする。① 要求は予算年度 N−1 行の翌年度要求額、②③④ は予算年度 N 行、⑥ の次年度要求は予算年度 N+1 行の翌年度要求額（= FY N+2）。

## 参照

- 列の仕様と品質問題: `docs/data-survey.md`
