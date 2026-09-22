# 引き継ぎ書: Phase 3b（2026 年度シート公開後の作業）

作成: 2026-09-22。Phase 3a 完了時点の状態と、再開時の手順をまとめる。

## 現在の状態

- リポジトリ: https://github.com/kwrkb/zailoop（プライベート）。`main` が最新
- 完了: Phase 1（CLI）、Phase 2（静的サイト）、Phase 3a（2024・2025 年度の結合とループ検証）
- 手元データ: `data/csv/` に 2024・2025 年度の 15 ファイルずつ（コミット対象外。`zailoop fetch --year <年度>` で再取得できる）
- ドキュメントの優先順: VISION.md > PLAN.md > コード > NOTES.md。列の対応は `docs/data-survey.md`

## 再開の手順

```sh
cd ~/code/zailoop
go build ./... && go test ./...                  # Go 側の健全性
cd web && npm install && npm run verify && cd .. # フロント（Node 24、fnm）
go run ./cmd/zailoop fetch --year 2026           # 公開されていれば 15 ファイルを取得
go run ./cmd/zailoop show 884 --years 2024,2025,2026
go run ./cmd/zailoop build --years 2024,2025,2026 --out site
```

`site/index.html` はブラウザで直接開ける（file:// 可）。

## 2026 年度シートが出たら最初に確認すること

1. **ヘッダが 2025 年度と一致するか。** `docs/data-survey.md` §2 の 15 ファイルの列名と比べる。違えば `internal/rs` の必須列定義（各 `*Builder` の `columns`）と `types.go` を見直す。確認用の 1 行:
   ```sh
   for p in 1-1 1-2 1-3 1-4 1-5 2-1 2-2 3-1 3-2 4-1 5-1 5-2 5-3 5-4 6-1; do diff <(head -1 data/csv/${p}_RS_2025_*.csv) <(head -1 data/csv/${p}_RS_2026_*.csv) >/dev/null || echo "DIFF $p"; done
   ```
2. **予算事業ID が各ファイルで連続・昇順か。** 崩れていると `rs.Each` が `ErrUnordered` で止まる。その場合は VISION 7 の「覆す条件」に該当するので、map 保持方式（`LoadAll`）を足す判断をする
3. **ZIP のエントリ名の文字コード。** 2025 年度は Shift_JIS だった。`fetch` は ZIP 名から CSV 名を決めるので影響しないはずだが、展開後に `data/csv` の名前が正しいか見る
4. **予算年度の範囲。** 2025 年度は 2021〜2025 だった。2026 年度で 2021 が落ちるなら、`Track` の推移表はそのまま動く（和集合を取る）
5. **公開時期の注記。** サイトの案内では「2026 年 9 月下旬から順次公開」。順次なので、初回取得時に事業数が少ない可能性がある。`1-2` の行数を 2025 年度（6,061 行、5,794 事業）と比べる

## Phase 3b でやること（PLAN.md）

- `--years 2024,2025,2026` で 3 年連続の推移とループ検証。`Track` は年度数に依存しない設計なので、原則そのまま動く。`Loops` は 2024→2025、2025→2026 の 2 本が Closed になる
- 一覧 JSON のスパークライン `yr` は `Meta.Years`（基準年度 − 3 〜 基準年度）に固定している。3 年結合で 2021 を落とすか 6 年にするかを決める（`internal/site/index.go` の `newIndexBuilder`）
- ID が変わった継続事業の候補提示。2025 年度にしかない「前年度事業」50 件が対象。名称・所管・関連事業（`1-5`）の一致で候補を出し、**自動結合はしない**（VISION 11）
- ヘッダ差分チェックを `fetch` 後に自動で出す仕組み（上の 1 行を Go に移す）

## 設計上の約束（変えるなら VISION の改訂が必要）

- 複数年度の結合キーは予算事業ID のみ。重なる予算年度は新しいシートを正（VISION 11）
- ループ検証は「シート S の反映状況」と「S+1 シートの FY S+1 当初」の増減で判定（VISION 12）。実装は `internal/lifecycle/track.go` の `judgeLoop`
- 断絶判定の閾値は `zailoop build` の引数。フロントでは再判定しない（VISION 9）
- Go は stdlib のみ。フロントの依存は typescript と esbuild だけで、`internal/site/assets/app.js` はコミットする（VISION 8）

## 既知の注意点

- 2024 年度にしかない 433 事業は、複数年度ビルドでも最新シートが 2024 のため実績年度が FY2023 になる。一覧では「2024年度まで」のバッジで区別している（JSON の `sy`）
- 「成果実績なし」の兆候は 1,547 件と多い。目標年度が先で FY N の実績がない指標が多いためで、閾値ではなく定義の問題
- `4-1` の反映額に正の値が入る事業がある（例: 884 の 2025 年度シートは「縮減」で +7,199,000）。データ側の記入ゆれとして扱い、判定には当初予算の増減だけを使う

## 分担の記録

- Codex: `internal/rs`（CSV 読み取り、`Each`、`Multi`）と、各ターン終了時のストップゲートレビュー
- Claude: それ以外。設計判断は LESSONS.md に「却下した案 / 決め手 / 覆す条件」で記録している
