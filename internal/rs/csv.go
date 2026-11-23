package rs

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// columnAliases は列名の新旧対応。キーは読み取り側が使う名前（2025 年度シートまでの列名）、
// 値は後の年度で置き換わった列名。ヘッダにキーの列がなければ値の列を探し、
// 値が複数あるとき（1 列が分割された場合）は行ごとに空でない最初の値を読む。
var columnAliases = map[string][]string{
	// 2-1（2026 年度シート）: 合計列の名前に「一般会計・特別会計」が入った。
	"当初予算（合計）":      {"当初予算（一般会計・特別会計合計）"},
	"補正予算（合計）":      {"補正予算（一般会計・特別会計合計）"},
	"前年度からの繰越し（合計）": {"前年度から繰越し（一般会計・特別会計合計）"},
	"予備費等（合計）":      {"予備費等（一般会計・特別会計合計）"},
	"計（歳出予算現額合計）":   {"計（一般会計・特別会計歳出予算現額合計）"},
	"執行額（合計）":       {"執行額（一般会計・特別会計合計）"},
	"翌年度要求額（合計）":    {"翌年度要求額（一般会計・特別会計合計）"},
	// 3-1（2026 年度シート）: アクティビティの内容と、活動目標・成果目標が別の列になった。
	"アクティビティ／活動目標／成果目標": {"アクティビティの内容", "活動目標／成果目標"},
	// 5-1（2026 年度シート）: 単位が列名に入った。
	"落札率": {"落札率（％）"},
}

// csvRow is valid only during a scan callback; the CSV reader reuses its slice.
type csvRow struct {
	columns map[string][]int
	fields  []string
	err     error
}

// text は列の値を返す。ヘッダにない列（任意列）は空文字列を返す。
func (r *csvRow) text(name string) string {
	indexes := r.columns[name]
	for _, i := range indexes {
		if strings.TrimSpace(r.fields[i]) != "" {
			return r.fields[i]
		}
	}
	if len(indexes) > 0 {
		return r.fields[indexes[0]]
	}
	return ""
}

// has は列がヘッダにあるか（別名を含む）を返す。
func (r *csvRow) has(name string) bool { return len(r.columns[name]) > 0 }

func (r *csvRow) hasAny(names []string) bool {
	for _, name := range names {
		if strings.TrimSpace(r.text(name)) != "" {
			return true
		}
	}
	return false
}

func parseYen(value string) (Yen, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Yen{}, nil
	}
	integer := value
	if whole, fraction, ok := strings.Cut(integer, "."); ok {
		if fraction == "" || strings.Trim(fraction, "0") != "" {
			return Yen{}, fmt.Errorf("整数ではありません: %q", value)
		}
		integer = whole
	}
	integer = strings.ReplaceAll(integer, ",", "")
	n, err := strconv.ParseInt(integer, 10, 64)
	if err != nil {
		return Yen{}, err
	}
	return Yen{Value: n, Valid: true}, nil
}

func (r *csvRow) fail(name string, err error) {
	if err != nil && r.err == nil {
		r.err = fmt.Errorf("列 %q の値 %q: %w", name, r.text(name), err)
	}
}

func (r *csvRow) yen(name string) Yen {
	value, err := parseYen(r.text(name))
	r.fail(name, err)
	return value
}

func (r *csvRow) year(name string) int {
	value, err := strconv.ParseInt(strings.TrimSpace(r.text(name)), 10, strconv.IntSize)
	r.fail(name, err)
	return int(value)
}

func (r *csvRow) ratio(name string) Ratio {
	value := strings.TrimSpace(r.text(name))
	if value == "" {
		return Ratio{}
	}
	n, err := strconv.ParseFloat(value, 64)
	if err == nil && (math.IsNaN(n) || math.IsInf(n, 0)) {
		err = fmt.Errorf("有限の数値ではありません")
	}
	r.fail(name, err)
	return Ratio{Value: n, Valid: err == nil}
}

type csvFile struct {
	path    string
	f       *os.File
	reader  *csv.Reader
	idIndex int              // 予算事業ID の列位置
	columns map[string][]int // 列名（別名を含む）→ 列位置
}

func (f *csvFile) row(record []string) csvRow { return csvRow{columns: f.columns, fields: record} }

func (f *csvFile) Close() error { return f.f.Close() }

func (d Dir) openCSV(number string, required []string) (*csvFile, error) {
	pattern := filepath.Join(d.Path, fmt.Sprintf("%s_RS_%d_*.csv", number, d.Year))
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("CSV の検索 %q: %w", pattern, err)
	}
	if len(paths) != 1 {
		return nil, fmt.Errorf("CSV %q: 対象ファイルは1件必要です（%d件）", pattern, len(paths))
	}
	path := paths[0]
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("CSV を開く: %w", err)
	}
	ready := false
	defer func() {
		if !ready {
			f.Close()
		}
	}()

	input := bufio.NewReader(f)
	if prefix, _ := input.Peek(3); string(prefix) == "\xef\xbb\xbf" {
		_, _ = input.Discard(3)
	}
	reader := csv.NewReader(input)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("%s: ヘッダ: %w", path, err)
	}
	columns := make(map[string][]int, len(header))
	for i, name := range header {
		if _, exists := columns[name]; exists {
			return nil, fmt.Errorf("%s: ヘッダ %q が重複しています", path, name)
		}
		columns[name] = []int{i}
	}
	for name, aliases := range columnAliases {
		if _, exists := columns[name]; exists {
			continue
		}
		for _, alias := range aliases {
			if i, ok := columns[alias]; ok {
				columns[name] = append(columns[name], i...)
			}
		}
	}
	for _, name := range append([]string{"予算事業ID"}, required...) {
		if _, exists := columns[name]; !exists {
			return nil, fmt.Errorf("%s: 必須ヘッダ %q がありません", path, name)
		}
	}
	ready = true
	return &csvFile{path: path, f: f, reader: reader, idIndex: columns["予算事業ID"][0], columns: columns}, nil
}

// scan validates the schema before visiting only the requested project's rows.
// It reads to EOF, including records after the last matching ID.
// 読んだ CSV のファイル名（ディレクトリを含まない）を返す。
func (d Dir) scan(number, id string, required []string, visit func(*csvRow) error) (string, error) {
	file, err := d.openCSV(number, required)
	if err != nil {
		return "", err
	}
	defer file.Close()
	name := filepath.Base(file.path)
	path, reader := file.path, file.reader
	for {
		record, err := reader.Read()
		if err == io.EOF {
			return name, nil
		}
		if err != nil {
			return "", fmt.Errorf("%s: CSV 読み取り: %w", path, err)
		}
		if strings.TrimSpace(record[file.idIndex]) != id {
			continue
		}
		row := file.row(record)
		err = visit(&row)
		if row.err != nil {
			err = row.err
		}
		if err != nil {
			line, _ := reader.FieldPos(0)
			return "", fmt.Errorf("%s:%d: 予算事業ID %s: %w", path, line, id, err)
		}
	}
}
