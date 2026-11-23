//go:build ignore

// 予算事業ID が変わったとみられる継続事業について、前年度にあって消えた ID から前身の候補を出す。
// 人が裏取りして確認記録（checks/<ID>.md）に書くための補助で、サイト・兆候・年度の結合には使わない
// （VISION 11・14。LESSONS 2026-11-23）。
// 使い方: go run .claude/id-candidates.go [-dir data/csv] [-years 2024,2025,2026] [-min 0.5] [-top 3]
// 出力は TSV。先頭の「# 」の行は出典と注記（grep -v '^#' で外せる）。
//
// 対象: 隣り合う年度 (a, b) で、b の 1-2 の事業区分が「前年度事業」なのに、a 以前の読み込んだどの年度の 1-2 にもない ID。
// 候補: a の 1-2 にあって、b 以後の読み込んだどの年度の 1-2 にもない ID（消えた ID）。根拠の種類（列「根拠」）:
//   - sheet:旧事業番号  b の 1-2「旧事業番号」を「、」「,」空白で区切り、全体が数字の項目（先頭の 0 を除く）が候補の ID と一致。
//     「2121-府-20-0128」のような旧形式の番号は数字以外を含むので照合しない
//   - sheet:1-5         a の 1-5 で候補が対象を統合先・分割先に、または b の 1-5 で対象が候補を統合元・分割元に書いている
//   - 推定:名称         事業名（全角英数を半角に、空白と末尾の括弧書きを除く）の文字 2-gram の Jaccard 係数が -min 以上。
//     府省はまたいで比べ、係数の高い順に -top 件まで
//
// sheet: はシートの記載、推定: は zailoop の推定で、どちらも同じ事業だと確かめたものではない。
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

type project struct {
	id, org, name, kind, oldNo string
}

type link struct{ from, to, rel string } // 1-5: from の行に to が rel として書かれている

// norm は全角英数・記号を半角に、全角空白を半角に寄せる（事業名に出る範囲の NFKC）。
func norm(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 0xFF01 && r <= 0xFF5E:
			return r - 0xFEE0
		case r == 0x3000, r == 0x00A0:
			return ' '
		}
		return r
	}, strings.TrimSpace(s))
}

var trailingParen = regexp.MustCompile(`\([^()]*\)\s*$`)

// nameKey は比べるための事業名。末尾の括弧書き（「（情報通信技術調達等適正・効率化推進費）」など）と空白を除く。
func nameKey(s string) string {
	s = norm(s)
	for {
		t := trailingParen.ReplaceAllString(s, "")
		if t == s || strings.TrimSpace(t) == "" {
			break
		}
		s = strings.TrimSpace(t)
	}
	return strings.Join(strings.Fields(s), "")
}

func bigrams(s string) map[string]bool {
	r := []rune(s)
	out := map[string]bool{}
	if len(r) == 1 {
		out[s] = true
	}
	for i := 0; i+1 < len(r); i++ {
		out[string(r[i:i+2])] = true
	}
	return out
}

func jaccard(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for k := range a {
		if b[k] {
			inter++
		}
	}
	return float64(inter) / float64(len(a)+len(b)-inter)
}

// each は 1 ファイルを読み、列名で引ける行を渡す。
func each(dir, prefix string, year int, fn func(col func(string) string)) {
	paths, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%s_RS_%d_*.csv", prefix, year)))
	if len(paths) != 1 {
		fail(fmt.Errorf("%s の %d 年度 CSV が 1 つに定まらない: %v", prefix, year, paths))
	}
	f, err := os.Open(paths[0])
	if err != nil {
		fail(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	head, err := r.Read()
	if err != nil {
		fail(err)
	}
	idx := map[string]int{}
	for i, h := range head {
		idx[strings.TrimPrefix(h, "\xef\xbb\xbf")] = i
	}
	for {
		row, err := r.Read()
		if err == io.EOF {
			return
		}
		if err != nil {
			fail(err)
		}
		fn(func(name string) string {
			i, ok := idx[name]
			if !ok {
				fail(fmt.Errorf("%s に列 %q がない", prefix, name))
			}
			if i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		})
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func loadProjects(dir string, year int) map[string]*project {
	out := map[string]*project{}
	each(dir, "1-2", year, func(col func(string) string) {
		id := col("予算事業ID")
		if _, ok := out[id]; ok {
			return
		}
		out[id] = &project{id: id, org: col("府省庁"), name: col("事業名"), kind: col("事業区分"), oldNo: col("旧事業番号")}
	})
	return out
}

func loadLinks(dir string, year int) []link {
	var out []link
	each(dir, "1-5", year, func(col func(string) string) {
		if rel := col("関連性"); rel != "" {
			out = append(out, link{from: col("予算事業ID"), to: col("関連事業の事業ID"), rel: rel})
		}
	})
	return out
}

// oldIDs は旧事業番号のうち、全体が数字の項目を先頭の 0 を除いて返す。2026 年度シートは値の先頭に ' が付く。
func oldIDs(s string) []string {
	var out []string
	for _, f := range strings.FieldsFunc(norm(strings.TrimPrefix(strings.TrimSpace(s), "'")), func(r rune) bool {
		return r == '、' || r == ',' || r == ' ' || r == '・' || r == '/'
	}) {
		if _, err := strconv.Atoi(f); err == nil {
			if id := strings.TrimLeft(f, "0"); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

type candidate struct {
	p     *project
	basis []string
	score float64
}

func main() {
	dir := flag.String("dir", "data/csv", "CSV のディレクトリ")
	yearsFlag := flag.String("years", "2024,2025,2026", "比べる事業年度（隣り合う年度ごとに出す）")
	min := flag.Float64("min", 0.5, "推定:名称 に出す Jaccard 係数の下限")
	top := flag.Int("top", 3, "推定:名称 の候補の上限（対象 1 件あたり）")
	flag.Parse()

	var years []int
	for _, s := range strings.Split(*yearsFlag, ",") {
		y, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			fail(fmt.Errorf("-years: %w", err))
		}
		years = append(years, y)
	}
	sort.Ints(years)

	fmt.Println("# " + render.Attribution)
	fmt.Println("# " + render.Disclaimer)
	fmt.Println("# 候補は人が裏取りするための手がかりで、同じ事業だと確かめたものではない。sheet: はシートの記載、推定: は zailoop の推定")
	fmt.Println(strings.Join([]string{"年度", "対象ID", "対象府省庁", "対象事業名", "候補ID", "候補府省庁", "候補事業名", "根拠", "係数"}, "\t"))

	projects := map[int]map[string]*project{}
	links := map[int][]link{}
	for _, y := range years {
		projects[y], links[y] = loadProjects(*dir, y), loadLinks(*dir, y)
	}
	inAny := func(ys []int, id string) bool {
		for _, y := range ys {
			if _, ok := projects[y][id]; ok {
				return true
			}
		}
		return false
	}

	for i := 0; i+1 < len(years); i++ {
		a, b := years[i], years[i+1]
		pa, pb := projects[a], projects[b]
		la, lb := links[a], links[b]

		var gone []*project
		for id, p := range pa {
			if !inAny(years[i+1:], id) {
				gone = append(gone, p)
			}
		}
		var targets []*project
		for id, p := range pb {
			if !inAny(years[:i+1], id) && p.kind == "前年度事業" {
				targets = append(targets, p)
			}
		}
		sort.Slice(targets, func(i, j int) bool { return atoi(targets[i].id) < atoi(targets[j].id) })
		goneByID := map[string]*project{}
		for _, g := range gone {
			goneByID[g.id] = g
		}

		counts := map[string]int{}
		for _, t := range targets {
			byID := map[string]*candidate{}
			add := func(g *project, basis string) {
				c, ok := byID[g.id]
				if !ok {
					c = &candidate{p: g, score: jaccard(bigrams(nameKey(t.name)), bigrams(nameKey(g.name)))}
					byID[g.id] = c
				}
				for _, b := range c.basis {
					if b == basis {
						return
					}
				}
				c.basis = append(c.basis, basis)
			}
			for _, id := range oldIDs(t.oldNo) {
				if g, ok := goneByID[id]; ok {
					add(g, "sheet:旧事業番号")
				}
			}
			for _, l := range la {
				if g, ok := goneByID[l.from]; ok && l.to == t.id && (l.rel == "統合先" || l.rel == "分割先") {
					add(g, "sheet:1-5")
				}
			}
			for _, l := range lb {
				if g, ok := goneByID[l.to]; ok && l.from == t.id && (l.rel == "統合元" || l.rel == "分割元") {
					add(g, "sheet:1-5")
				}
			}
			tk := bigrams(nameKey(t.name))
			var scored []candidate
			for _, g := range gone {
				if s := jaccard(tk, bigrams(nameKey(g.name))); s >= *min {
					scored = append(scored, candidate{p: g, score: s})
				}
			}
			sort.Slice(scored, func(i, j int) bool {
				if scored[i].score != scored[j].score {
					return scored[i].score > scored[j].score
				}
				return atoi(scored[i].p.id) < atoi(scored[j].p.id)
			})
			for k := 0; k < len(scored) && k < *top; k++ {
				add(scored[k].p, "推定:名称")
			}

			var cs []*candidate
			for _, c := range byID {
				cs = append(cs, c)
			}
			sort.Slice(cs, func(i, j int) bool {
				si, sj := strings.HasPrefix(cs[i].basis[0], "sheet:"), strings.HasPrefix(cs[j].basis[0], "sheet:")
				if si != sj {
					return si
				}
				return cs[i].score > cs[j].score
			})
			pair := fmt.Sprintf("%d→%d", a, b)
			if len(cs) == 0 {
				counts["候補なし"]++
				fmt.Println(strings.Join([]string{pair, t.id, t.org, t.name, "", "", "", "候補なし", ""}, "\t"))
				continue
			}
			sheet := false
			for _, c := range cs {
				for _, b := range c.basis {
					sheet = sheet || strings.HasPrefix(b, "sheet:")
				}
				fmt.Println(strings.Join([]string{pair, t.id, t.org, t.name, c.p.id, c.p.org, c.p.name, strings.Join(c.basis, ","), fmt.Sprintf("%.2f", c.score)}, "\t"))
			}
			if sheet {
				counts["シートの記載あり"]++
			} else {
				counts["推定のみ"]++
			}
		}
		fmt.Fprintf(os.Stderr, "%d→%d: 対象 %d（シートの記載あり %d / 推定のみ %d / 候補なし %d）、消えた ID %d\n",
			a, b, len(targets), counts["シートの記載あり"], counts["推定のみ"], counts["候補なし"], len(gone))
	}
	fmt.Fprintln(os.Stderr, render.Attribution)
	fmt.Fprintln(os.Stderr, render.Disclaimer)
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
