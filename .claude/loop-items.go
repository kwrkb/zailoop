//go:build ignore

// ループ検証が「反映要確認」の事業について、2-2 の当初予算を目ごとに並べ、どの目が増えたかを出す。
// 確認記録（checks/<ID>.md）の sheet_ref の候補を作るための補助で、兆候にもサイトにも使わない（LESSONS 2026-11-25）。
// 使い方: go run .claude/loop-items.go [-dir data/csv] [-years 2024,2025,2026] [-latest] [-summary] [予算事業ID ...]
// 出力は TSV。先頭の「# 」の行は出典と注記（grep -v '^#' で外せる）。
// ID を省くと反映要確認のループがある事業すべて。-latest は一覧に出る事業（最新のループが反映要確認）だけ。
//
// 行の種類（1 列目）:
//   - loop: シート S のループ 1 つ。2-1 の当初・要求と 2-2 の計の照合、要望額、4-1 の反映額
//   - item: そのループの目 1 つ。FY S の当初（S シート）、FY S+1 の要求（S シート）、FY S+1 の当初（S+1 シート）
//
// 決まり:
//   - 反映要確認かどうかは lifecycle.Track の判定をそのまま使い、ここでは判定し直さない
//   - 2-2 は予算種別「当初予算」の行だけを足す。同じ会計・勘定・組織・項・目の行は足し合わせる
//   - レコード番号（rNNNN）は CSV の論理レコード番号で、ヘッダを 1 とする
//   - 2-2 の計が 2-1 と合わないループは、目ごとの分解をそのまま根拠にしない（列「照合」を見る）
//   - 「反映額と同額の減」は、4-1 の反映額の絶対値と、減った目の額が同じもの。同額であることしか言わない
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/render"
	"github.com/kwrkb/zailoop/internal/rs"
)

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

// yen は金額の欄を読む。空欄は 0（2026 年度形式は 0 を空欄で書く）。読めなければ ok が false。
func yen(s string) (int64, bool) {
	s = strings.ReplaceAll(s, ",", "")
	if s == "" {
		return 0, true
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		return v, true
	}
	f, err := strconv.ParseFloat(s, 64)
	return int64(f), err == nil
}

type loopKey struct {
	id    string
	sheet int
}

type loopInfo struct {
	name, ministry string
	latest         bool // 最新のループ（一覧に出る判定）か
	lp             lifecycle.Loop
	demand         int64    // 2-1 の会計別の行の要望額の計（S シートの FY S）
	reflected      []string // 4-1 の反映額（会計ごと、記載どおり）
	reflectedAbs   []int64
}

// cell は 1 つの目の 1 つの値（同じ目の行は足し合わせる）。
type cell struct {
	amount int64
	recs   []int
	bad    bool
}

func (c *cell) add(v int64, ok bool, rec int) {
	c.amount += v
	c.recs = append(c.recs, rec)
	if !ok {
		c.bad = true
	}
}

func (c *cell) String() string {
	if c == nil || len(c.recs) == 0 {
		return "-"
	}
	if c.bad {
		return "読めない"
	}
	return strconv.FormatInt(c.amount, 10)
}

func (c *cell) ref() string {
	if c == nil || len(c.recs) == 0 {
		return "-"
	}
	out := make([]string, len(c.recs))
	for i, r := range c.recs {
		out[i] = fmt.Sprintf("r%d", r)
	}
	return strings.Join(out, ",")
}

type item struct {
	initial, request, next cell // FY S 当初（S）、FY S+1 要求（S）、FY S+1 当初（S+1）
	note                   string
}

func yenStr(y rs.Yen) string {
	if !y.Valid {
		return "-"
	}
	return strconv.FormatInt(y.Value, 10)
}

func match(a rs.Yen, b int64, rows bool) string {
	switch {
	case !rows:
		return "2-2 に行がない"
	case !a.Valid:
		return "2-1 が空欄"
	case a.Value == b:
		return "一致"
	}
	return fmt.Sprintf("不一致（差 %d）", b-a.Value)
}

func main() {
	dir := flag.String("dir", "data/csv", "CSV のディレクトリ")
	yearsFlag := flag.String("years", "2024,2025,2026", "読むシートの事業年度（カンマ区切り）")
	latestOnly := flag.Bool("latest", false, "最新のループが反映要確認の事業だけ（一覧に出る事業）")
	summary := flag.Bool("summary", false, "loop の行だけ出す")
	flag.Parse()
	want := map[string]bool{}
	for _, a := range flag.Args() {
		want[a] = true
	}

	var dirs []rs.Dir
	for _, s := range strings.Split(*yearsFlag, ",") {
		y, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			fmt.Fprintf(os.Stderr, "-years が読めない: %q\n", s)
			os.Exit(2)
		}
		dirs = append(dirs, rs.Dir{Path: *dir, Year: y})
	}

	loops := map[loopKey]*loopInfo{}
	needYear := map[int]bool{}
	err := rs.Multi{Dirs: dirs}.Each(func(sheets []*rs.Sheet) error {
		tl := lifecycle.Track(sheets, lifecycle.Thresholds{})
		if len(want) > 0 && !want[tl.ID] {
			return nil
		}
		last := -1
		for i, lp := range tl.Loops {
			if lp.Closed {
				last = i
			}
		}
		for i, lp := range tl.Loops {
			if lp.Verdict != lifecycle.VerdictContradiction {
				continue
			}
			if *latestOnly && i != last {
				continue
			}
			info := &loopInfo{latest: i == last, lp: lp}
			for _, s := range sheets {
				if s.FiscalYear != lp.SheetYear {
					continue
				}
				info.name, info.ministry = s.Project.Name, s.Project.Ministry
				if b := s.Budget(lp.SheetYear); b != nil {
					for _, a := range b.Accounts {
						if a.Demand.Valid {
							info.demand += a.Demand.Value
						}
					}
				}
				add := func(label string, y rs.Yen) {
					if !y.Valid {
						return
					}
					info.reflected = append(info.reflected, fmt.Sprintf("%s=%d", label, y.Value))
					if y.Value < 0 {
						info.reflectedAbs = append(info.reflectedAbs, -y.Value)
					} else if y.Value > 0 {
						info.reflectedAbs = append(info.reflectedAbs, y.Value)
					}
				}
				add("一般会計", s.Evaluation.ReflectedGeneral)
				for _, sp := range s.Evaluation.ReflectedSpecial {
					label := sp.Account
					if sp.Subaccount != "" {
						label += "／" + sp.Subaccount
					}
					add(label, sp.Amount)
				}
			}
			loops[loopKey{tl.ID, lp.SheetYear}] = info
			needYear[lp.SheetYear] = true
			needYear[lp.SheetYear+1] = true
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 2-2 を読む。S シートの FY S の行は S のループの当初と要求に、
	// S シートの FY S の行は S−1 のループの「翌年度の当初」にもなる。
	items := map[loopKey]map[string]*item{}
	get := func(k loopKey, name string) *item {
		if items[k] == nil {
			items[k] = map[string]*item{}
		}
		if items[k][name] == nil {
			items[k][name] = &item{}
		}
		return items[k][name]
	}
	var years []int
	for y := range needYear {
		years = append(years, y)
	}
	sort.Ints(years)
	for _, sy := range years {
		each(*dir, "2-2", sy, func(rec int, col func(string) string) {
			id := col("予算事業ID")
			cur, prev := loops[loopKey{id, sy}], loops[loopKey{id, sy - 1}]
			if cur == nil && prev == nil {
				return
			}
			if col("予算種別") != "当初予算" || col("予算年度") != strconv.Itoa(sy) {
				return
			}
			acct := col("会計")
			if s := col("勘定"); s != "" {
				acct += "／" + s
			}
			name := strings.Join([]string{acct, col("組織・勘定"), col("項"), col("目")}, "\t")
			amt, okA := yen(col("予算額（歳出予算項目ごと）"))
			req, okR := yen(col("翌年度要求額（歳出予算項目ごと）"))
			note := col("歳出予算項目の補足情報")
			if cur != nil {
				it := get(loopKey{id, sy}, name)
				it.initial.add(amt, okA, rec)
				it.request.add(req, okR, rec)
				if note != "" && !strings.Contains(it.note, note) {
					it.note = strings.TrimPrefix(it.note+"；"+note, "；")
				}
			}
			if prev != nil {
				it := get(loopKey{id, sy - 1}, name)
				it.next.add(amt, okA, rec)
			}
		})
	}

	keys := make([]loopKey, 0, len(loops))
	for k := range loops {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := strconv.Atoi(keys[i].id)
		b, _ := strconv.Atoi(keys[j].id)
		if a != b {
			return a < b
		}
		return keys[i].sheet < keys[j].sheet
	})

	fmt.Println("# " + render.Attribution)
	fmt.Println("# " + render.Disclaimer)
	fmt.Println("# loop\t予算事業ID\tシート\t府省庁\t最新のループ\t反映状況\t2-1 当初 FY S\t2-2 計\t照合\t2-1 要求 FY S+1\t2-2 要求の計\t照合\t2-1 要望額\t2-1 当初 FY S+1\t2-2 計\t照合\t増減\t4-1 反映額\t反映額と同額の減\t事業名")
	fmt.Println("# item\t予算事業ID\tシート\t会計\t組織・勘定\t項\t目\t当初 FY S\t場所\t要求 FY S+1\t当初 FY S+1\t場所\t増減\t補足情報")
	count := map[string]int{}
	for _, k := range keys {
		info := loops[k]
		lp := info.lp
		var names []string
		var sumInit, sumReq, sumNext int64
		var hasInit, hasNext bool
		for name, it := range items[k] {
			names = append(names, name)
			sumInit += it.initial.amount
			sumReq += it.request.amount
			sumNext += it.next.amount
			hasInit = hasInit || len(it.initial.recs) > 0
			hasNext = hasNext || len(it.next.recs) > 0
		}
		sort.Strings(names)
		var same []string
		for _, name := range names {
			it := items[k][name]
			dec := it.initial.amount - it.next.amount
			for _, r := range info.reflectedAbs {
				if dec > 0 && dec == r {
					same = append(same, name[strings.LastIndex(name, "\t")+1:])
				}
			}
		}
		mInit := match(lp.Initial, sumInit, hasInit)
		mNext := match(lp.NextInitial, sumNext, hasNext)
		switch {
		case mInit == "一致" && mNext == "一致":
			count["2-2 の計が 2 年とも 2-1 と一致"]++
		default:
			count["2-2 の計が 2-1 と合わない年度がある"]++
		}
		latest := ""
		if info.latest {
			latest = "○"
		}
		fmt.Printf("loop\t%s\t%d\t%s\t%s\t%s\t%s\t%d\t%s\t%s\t%d\t%s\t%d\t%s\t%d\t%s\t%d\t%s\t%s\t%s\n",
			k.id, k.sheet, info.ministry, latest, lp.Reflection,
			yenStr(lp.Initial), sumInit, mInit,
			yenStr(lp.NextRequest), sumReq, match(lp.NextRequest, sumReq, hasInit),
			info.demand,
			yenStr(lp.NextInitial), sumNext, mNext,
			lp.NextInitial.Value-lp.Initial.Value,
			strings.Join(info.reflected, "；"), strings.Join(same, "；"), info.name)
		if *summary {
			continue
		}
		for _, name := range names {
			it := items[k][name]
			fmt.Printf("item\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
				k.id, k.sheet, name,
				it.initial.String(), it.initial.ref(), it.request.String(),
				it.next.String(), it.next.ref(),
				it.next.amount-it.initial.amount, it.note)
		}
	}
	fmt.Fprintf(os.Stderr, "反映要確認のループ %d 件\n", len(keys))
	for _, s := range []string{"2-2 の計が 2 年とも 2-1 と一致", "2-2 の計が 2-1 と合わない年度がある"} {
		fmt.Fprintf(os.Stderr, "  %s: %d\n", s, count[s])
	}
	fmt.Fprintln(os.Stderr, render.Attribution)
	fmt.Fprintln(os.Stderr, render.Disclaimer)
}
