//go:build ignore

// デジタル庁の個別システムの 1-2 備考に書かれた過年度の執行額を、5-1 の支出ブロック計と突き合わせる。
// 確認記録（checks/<ID>.md）の sheet_ref の候補を作るための補助で、兆候には使わない（LESSONS 2026-10-30）。
// 使い方: go run .claude/remarks-match.go [-dir data/csv] [-year 2025] [-all]
// 出力は TSV。先頭の「# 」の行は出典と注記（grep -v '^#' で外せる）。
// 既定では備考の定型が読めた事業のうち、判定が「一致」のものを省く。
//
// 照合の決まり（data-survey §6 の 15）:
//   - 備考の定型「「区分」／執行額：YYYY年度 N千円、…」の FY(シート年度−1) の項目を合計する。「ー」は 0
//   - 5-1 は 5-2 で「担当組織からの支出」が TRUE のブロックの合計（下流のブロックは足さない）
//   - 備考は項目ごとに千円未満を丸めているので、項目数 × 1,000 円までの差は一致とする
//   - 5-1 に載らない 5-2 の「国自らが支出する間接経費の金額」を足して合えば「一致（間接経費込み）」
//   - 足すブロックの一つでも 5-1 の合計支出額が未記載なら、残りで合っても照合しない（5-1 の一部が未記載）
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kwrkb/zailoop/internal/render"
)

var (
	segRe  = regexp.MustCompile(`「([^」]+)」\s*/?\s*執行額\s*:\s*([^/]+)`)
	itemRe = regexp.MustCompile(`(20\d\d)\s*年度\s*([0-9,]+|ー|-|－|―)\s*(千円|円)`)
)

// norm は全角英数・記号を半角に、半角かぎ括弧を全角に寄せる（備考に出る範囲の NFKC）。
func norm(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			return r - 0xFEE0
		case r == 0x3000, r == 0x00A0:
			return ' '
		case r == 0xFF62:
			return '「'
		case r == 0xFF63:
			return '」'
		}
		return r
	}, strings.TrimSpace(s))
}

func yen(s string) (int64, bool) {
	v, err := strconv.ParseInt(strings.ReplaceAll(norm(s), ",", ""), 10, 64)
	return v, err == nil
}

// each は 1 ファイルを読み、列名で引ける行と論理レコード番号（ヘッダを 1）を渡す。
func each(dir, prefix string, year int, fn func(rec int, col func(string) string)) {
	paths, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%s_RS_%d_*.csv", prefix, year)))
	if len(paths) != 1 {
		fmt.Fprintf(os.Stderr, "%s の %d 年度 CSV が 1 つに定まらない: %v\n", prefix, year, paths)
		os.Exit(1)
	}
	f, err := os.Open(paths[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[strings.TrimPrefix(h, "\xef\xbb\xbf")] = i
	}
	for rec := 2; ; rec++ {
		row, err := r.Read()
		if err == io.EOF {
			return
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fn(rec, func(name string) string {
			i, ok := idx[name]
			if !ok {
				fmt.Fprintf(os.Stderr, "%s に列 %q がない\n", prefix, name)
				os.Exit(1)
			}
			if i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		})
	}
}

type remark struct {
	rec      int
	org      string
	sum      int64 // 円
	items    int   // FY の項目数（「ー」も数える）
	amounts  int   // そのうち金額が書かれた項目数。「ー」は 0 円に置き換えない
	yenUnit  bool  // 「千円」でなく「円」と書いた項目がある
	hasFY    bool
	hasTempl bool
}

func main() {
	dir := flag.String("dir", "data/csv", "CSV のディレクトリ")
	year := flag.Int("year", 2025, "シート年度（照合するのは前年度の執行）")
	all := flag.Bool("all", false, "一致した事業も出す")
	flag.Parse()
	fy := *year - 1

	remarks := map[string]*remark{}
	each(*dir, "1-2", *year, func(rec int, col func(string) string) {
		id := col("予算事業ID")
		if _, done := remarks[id]; done {
			return // 主要経費で重複する行。備考は食い違わない（data-survey §6 の 13）
		}
		v := norm(col("備考"))
		if !strings.Contains(v, "執行額") {
			return
		}
		rm := &remark{rec: rec, org: col("府省庁")}
		for _, seg := range segRe.FindAllStringSubmatch(v, -1) {
			for _, it := range itemRe.FindAllStringSubmatch(seg[2], -1) {
				rm.hasTempl = true
				if y, _ := strconv.Atoi(it[1]); y != fy {
					continue
				}
				rm.hasFY = true
				rm.items++
				n, ok := yen(it[2])
				if !ok {
					continue // 「ー」
				}
				if it[3] == "千円" {
					n *= 1000
				} else {
					rm.yenUnit = true
				}
				rm.sum += n
				rm.amounts++
			}
		}
		if rm.hasTempl {
			remarks[id] = rm
		}
	})

	executed := map[string]int64{}
	each(*dir, "2-1", *year, func(_ int, col func(string) string) {
		if col("予算年度") != strconv.Itoa(fy) {
			return
		}
		if n, ok := yen(col("執行額（合計）")); ok && n >= executed[col("予算事業ID")] {
			executed[col("予算事業ID")] = n
		}
	})

	blocks := map[string]map[string]int64{}
	seen51 := map[string]map[string]bool{} // 合計支出額の有無によらず 5-1 に出たブロック
	each(*dir, "5-1", *year, func(_ int, col func(string) string) {
		id := col("予算事業ID")
		if _, ok := remarks[id]; !ok {
			return
		}
		if seen51[id] == nil {
			seen51[id] = map[string]bool{}
		}
		seen51[id][col("支出先ブロック番号")] = true
		if n, ok := yen(col("ブロックの合計支出額")); ok {
			if blocks[id] == nil {
				blocks[id] = map[string]int64{}
			}
			blocks[id][col("支出先ブロック番号")] = n
		}
	})

	roots := map[string]map[string]bool{}
	indirect := map[string]int64{}
	each(*dir, "5-2", *year, func(_ int, col func(string) string) {
		id := col("予算事業ID")
		if _, ok := remarks[id]; !ok {
			return
		}
		if strings.EqualFold(col("担当組織からの支出"), "TRUE") {
			if roots[id] == nil {
				roots[id] = map[string]bool{}
			}
			roots[id][col("支出先の支出先ブロック")] = true
		}
		if n, ok := yen(col("国自らが支出する間接経費の金額")); ok {
			indirect[id] += n
		}
	})

	ids := make([]string, 0, len(remarks))
	for id := range remarks {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, _ := strconv.Atoi(ids[i])
		b, _ := strconv.Atoi(ids[j])
		return a < b
	})

	fmt.Println("# " + render.Attribution)
	fmt.Println("# " + render.Disclaimer)
	fmt.Println("予算事業ID\t府省庁\t判定\t備考\t金額の項目数\t5-1\t間接経費\t2-1執行\tsheet_ref 候補")
	count := map[string]int{}
	for _, id := range ids {
		rm := remarks[id]
		// 足すのは 5-2 の担当組織からの支出のブロック。5-2 にない事業は 5-1 の全ブロック。
		// 合計支出額が未記載のブロックは支出 0 円ではなく欠測として扱う
		targets := roots[id]
		if len(targets) == 0 {
			targets = seen51[id]
		}
		var spent int64
		has51 := false
		var missing []string
		for no := range targets {
			if n, ok := blocks[id][no]; ok {
				spent += n
				has51 = true
			} else {
				missing = append(missing, no)
			}
		}
		sort.Strings(missing)
		tol := int64(1000 * rm.amounts)
		near := func(x int64) bool { d := x - rm.sum; return -tol <= d && d <= tol }

		verdict := "不一致"
		switch {
		case !rm.hasFY:
			verdict = fmt.Sprintf("備考に FY%d なし", fy)
		case rm.amounts == 0:
			verdict = fmt.Sprintf("備考の FY%d はすべてー", fy)
		case !has51:
			verdict = "5-1 なし"
		case len(missing) > 0:
			verdict = "5-1 の一部が未記載"
		case near(spent):
			verdict = "一致"
		case indirect[id] != 0 && near(spent+indirect[id]):
			verdict = "一致（間接経費込み）"
		}
		count[verdict]++
		if verdict == "一致" && !*all {
			continue
		}

		ref := "-"
		if verdict == "一致" || verdict == "一致（間接経費込み）" || verdict == "不一致" {
			v := "一致"
			if verdict == "不一致" {
				v = "不一致"
			}
			note := fmt.Sprintf("FY%d の執行額 %s 円（%d 項目、千円単位）", fy, comma(rm.sum), rm.amounts)
			if rm.items > rm.amounts {
				note += fmt.Sprintf("。「ー」の %d 項目は足していない", rm.items-rm.amounts)
			}
			if rm.yenUnit {
				note += "。単位を「円」と書いた項目がある"
			}
			switch {
			case verdict == "一致（間接経費込み）":
				note += fmt.Sprintf("。5-1 の担当組織からの支出 %s 円＋5-2 の間接経費 %s 円と合う", comma(spent), comma(indirect[id]))
			default:
				note += fmt.Sprintf("。5-1 の担当組織からの支出 %s 円（差 %s 円）", comma(spent), comma(spent-rm.sum))
			}
			ref = fmt.Sprintf("%d 1-2 備考 r%d | %s | %s", *year, rm.rec, v, note)
		}
		ex := "-"
		if n, ok := executed[id]; ok {
			ex = strconv.FormatInt(n, 10)
		}
		sum := "-"
		if rm.amounts > 0 {
			sum = strconv.FormatInt(rm.sum, 10)
		}
		s51 := "-"
		if has51 && len(missing) == 0 {
			s51 = strconv.FormatInt(spent, 10)
		} else if has51 {
			s51 = "未記載 " + strings.Join(missing, ",")
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%d\t%s\t%d\t%s\t%s\n", id, rm.org, verdict, sum, rm.amounts, s51, indirect[id], ex, ref)
	}
	keys := make([]string, 0, len(count))
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Fprintf(os.Stderr, "%d 年度シート: 備考の定型が読めた %d 件", *year, len(ids))
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, " / %s %d", k, count[k])
	}
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, render.Attribution)
	fmt.Fprintln(os.Stderr, render.Disclaimer)
}

func comma(n int64) string {
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return sign + s
}
