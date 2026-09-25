#!/usr/bin/env bash
# 確認記録（checks/<ID>.md）の書式を確かめる。形式は .claude/checks-format.md
# 使い方: .claude/check-records.sh checks/*.md   問題がなければ何も出さず 0、あれば一覧を出して 1
# kind の Signal キーは internal/lifecycle/signals.go と合わせる
# 空のファイルは awk の FNR==1 に届かず素通りするので、先に見る
out=$(
for f in "$@"; do if [ ! -f "$f" ]; then echo "$f: ファイルがない"; elif [ ! -s "$f" ]; then echo "$f: 空のファイル"; fi; done
awk -v signals='request_gap|low_execution|large_unused|cut|execution_without_budget|negative_unused|outcome_shortfall|outcome_overshoot|no_outcome_actual|reflection_contradicted|request_zeroed|amount_revised' '
function flush(){ if(f=="")return
  for(k in need) if(!(k in seen)) print f": 必須の項目がない: "k
  for(k in seen) if(seen[k]>1 && k!~/^(sheet|kind|sheet_ref|source|next)$/) print f": 繰り返せない項目: "k
  base=f; sub(/.*\//,"",base); sub(/\.md$/,"",base); if(id!=base) print f": id とファイル名が違う: "id
  if(!body) print f": 本文がない"
  delete seen }
function verdict(v){ return v~/^(一致|一部|不一致|候補)$/ }
BEGIN{split("id name sheet checked status kind question",r," "); for(i in r) need[r[i]]=1
  kinds="^(" signals "|all_zero|zero_with_spending|link_revised|id_moved|external_mismatch|other)$"}
FNR==1{flush(); f=FILENAME; h=1; body=0; id=""}
h && /^$/{h=0; next}
!h && /^## /{body=1}
h{ i=index($0,": "); if(!i){print f": ヘッダの行に「: 」がない: "$0; next}
  k=substr($0,1,i-1); v=substr($0,i+2); seen[k]++
  if(k!~/^(id|name|sheet|checked|status|kind|question|sheet_ref|source|next)$/) print f": 知らない項目: "k
  if(k=="id") id=v
  if(k=="sheet" && v!~/^20[0-9][0-9]$/) print f": sheet が年度でない: "v
  if(k=="checked" && v!~/^20[0-9][0-9]-[01][0-9]-[0-3][0-9]$/) print f": checked が日付でない: "v
  if(k=="status" && v!~/^(説明あり|一部|未解決)$/) print f": status の値: "v
  if(k=="kind" && v!~kinds) print f": kind の値: "v
  if(k=="sheet_ref"){ n=split(v,a," \\| ")
    if(n<3) print f": sheet_ref の欄が足りない("n"): "v
    if(a[1]!~/^20[0-9][0-9] [0-9]-[0-9] /) print f": sheet_ref の場所: "a[1]
    if(!verdict(a[2])) print f": sheet_ref の判定: "a[2] }
  if(k=="source"){ n=split(v,a," \\| ")
    if(n<4) print f": source の欄が足りない("n"): "v
    if(a[1]!~/^https?:\/\//) print f": source の URL: "a[1]
    if(a[2]!~/^([0-9]{4}([-,][0-9]{4})*\??|不明)$/) print f": source の資料の年度: "a[2]
    if(!verdict(a[3])) print f": source の判定: "a[3] } }
END{flush()}' "$@"
)
rc=$?
[ -z "$out" ] || { printf '%s\n' "$out"; exit 1; }
# awk 自体が失敗した（ファイルが読めないなど）ときは、出力が空でも失敗にする
[ "$rc" -eq 0 ] || exit "$rc"
