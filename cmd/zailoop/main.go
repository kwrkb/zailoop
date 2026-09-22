// zailoop は行政事業レビュー見える化サイトの CSV から、1 事業の予算ライフサイクルを表示する。
//
//	zailoop fetch [--year 2024] [--data data]
//	zailoop show <予算事業ID> [--year 2024] [--data data]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/kwrkb/zailoop/internal/fetch"
	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/render"
	"github.com/kwrkb/zailoop/internal/rs"
	"github.com/kwrkb/zailoop/internal/site"
)

const usage = `zailoop: 予算事業のライフサイクル（要求→成立→執行→決算→評価→翌年度反映）を表示する

使い方:
  zailoop fetch [--year 2024] [--data data]      配布 ZIP を取得して展開する（既存はスキップ）
  zailoop show <予算事業ID> [--year 2024] [--data data]
                                                  1 事業のライフサイクルを表示する
  zailoop build [--out site] [--year 2024] [--data data] [閾値オプション]
                                                  全事業の静的サイトを生成する
    閾値: --gap-ratio 0.5 --min-exec-rate 0.5 --min-unused 1000000000
          --unused-ratio 0.2 --outcome-low 80 --outcome-high 200

データは <data>/raw/ に ZIP、<data>/csv/ に CSV として置く。
` + render.Attribution + `
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "zailoop:", err)
		if errors.Is(err, errUsage) {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

var errUsage = errors.New("usage")

func run(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, usage)
		return errUsage
	}
	switch args[0] {
	case "fetch":
		return runFetch(args[1:])
	case "show":
		return runShow(args[1:])
	case "build":
		return runBuild(args[1:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		return nil
	}
	fmt.Fprint(os.Stderr, usage)
	return fmt.Errorf("%w: 不明なサブコマンド %q", errUsage, args[0])
}

func commonFlags(fs *flag.FlagSet) (*int, *string) {
	year := fs.Int("year", 2024, "事業年度")
	data := fs.String("data", "data", "データディレクトリ")
	return year, data
}

// parseInterspersed はフラグと位置引数の順序を問わずに解析し、位置引数を返す。
// 標準の flag は最初の位置引数で解析を止めるため、`show 884 --year 2024` のような
// README の書き方を受け付けるにはこれが必要。トークンを 1 つずつ見て、
// フラグなら（値を取るフラグは次のトークンも含めて）fs.Parse に渡し、それ以外は位置引数に積む。
// 単独の "--" はそこで終端し、以降はすべて位置引数。フラグの値として現れた "--" は終端にしない。
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(positional, args[i+1:]...), nil
		}
		if len(a) < 2 || a[0] != '-' || a == "-" {
			positional = append(positional, a)
			continue
		}
		name := strings.TrimLeft(a, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		f := fs.Lookup(name)
		if f == nil {
			// 未定義フラグは fs.Parse にエラーを出させる（-h も同様）
			return nil, fs.Parse([]string{a})
		}
		toks := []string{a}
		if !strings.Contains(a, "=") && !isBoolFlag(f) && i+1 < len(args) {
			i++
			toks = append(toks, args[i])
		}
		if err := fs.Parse(toks); err != nil {
			return nil, err
		}
	}
	return positional, nil
}

func isBoolFlag(f *flag.Flag) bool {
	b, ok := f.Value.(interface{ IsBoolFlag() bool })
	return ok && b.IsBoolFlag()
}

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	year, data := commonFlags(fs)
	pos, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return errUsage
	}
	if len(pos) != 0 {
		return fmt.Errorf("%w: fetch は位置引数を取りません: %v", errUsage, pos)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	_, err = fetch.Fetch(ctx, *year, filepath.Join(*data, "raw"), filepath.Join(*data, "csv"), fetch.Options{Log: os.Stderr})
	return err
}

func runShow(args []string) error {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	year, data := commonFlags(fs)
	pos, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return errUsage
	}
	if len(pos) != 1 {
		return fmt.Errorf("%w: show には予算事業ID を 1 つ指定する", errUsage)
	}
	id := pos[0]
	dir := rs.Dir{Path: filepath.Join(*data, "csv"), Year: *year}
	sheet, err := dir.LoadSheet(id)
	if err != nil {
		if errors.Is(err, rs.ErrNotFound) {
			return fmt.Errorf("予算事業ID %s は %d 年度のデータにありません", id, *year)
		}
		return err
	}
	lc := lifecycle.Build(sheet, lifecycle.Options{})
	return render.Text(os.Stdout, lc)
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	year, data := commonFlags(fs)
	out := fs.String("out", "site", "出力ディレクトリ")
	d := lifecycle.DefaultThresholds()
	var th lifecycle.Thresholds
	fs.Float64Var(&th.RequestGapRatio, "gap-ratio", d.RequestGapRatio, "当初/要求 がこれ未満で「要求と成立の乖離」")
	fs.Float64Var(&th.LowExecRate, "min-exec-rate", d.LowExecRate, "執行率がこれ未満で「低執行」")
	fs.Int64Var(&th.LargeUnusedYen, "min-unused", d.LargeUnusedYen, "不用相当額（円）がこれ以上で「大きな不用」")
	fs.Float64Var(&th.LargeUnusedRatio, "unused-ratio", d.LargeUnusedRatio, "不用相当額/現額 がこれ以上で「大きな不用」")
	fs.Float64Var(&th.OutcomeShortfall, "outcome-low", d.OutcomeShortfall, "アウトカム達成率（%）がこれ未満で「成果未達」")
	fs.Float64Var(&th.OutcomeOvershoot, "outcome-high", d.OutcomeOvershoot, "アウトカム達成率（%）がこれ超で「成果超過」")
	pos, err := parseInterspersed(fs, args)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return errUsage
	}
	if len(pos) != 0 {
		return fmt.Errorf("%w: build は位置引数を取りません: %v", errUsage, pos)
	}
	dir := rs.Dir{Path: filepath.Join(*data, "csv"), Year: *year}
	st, err := site.Build(dir, *out, site.Options{Year: *year, Thresholds: th, Log: os.Stderr})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%d 事業, %.1f MB, %s → %s\n", st.Projects, float64(st.Bytes)/1e6, st.Elapsed.Round(time.Millisecond), *out)
	return nil
}
