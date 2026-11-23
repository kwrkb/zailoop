package main

import (
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParseInterspersed(t *testing.T) {
	cases := []struct {
		args     []string
		wantPos  []string
		wantYear int
		wantData string
	}{
		{[]string{"884"}, []string{"884"}, 2024, "data"},
		{[]string{"--year", "2025", "884"}, []string{"884"}, 2025, "data"},
		{[]string{"884", "--year", "2025"}, []string{"884"}, 2025, "data"},
		{[]string{"884", "--year=2025", "--data", "d"}, []string{"884"}, 2025, "d"},
		{[]string{"--data", "d", "884", "--year", "2025"}, []string{"884"}, 2025, "d"},
		{[]string{"884", "--", "--year", "2025"}, []string{"884", "--year", "2025"}, 2024, "data"},
		{[]string{"--data", "--", "884"}, []string{"884"}, 2024, "--"},
		{[]string{"884", "--data", "--", "--year", "2025"}, []string{"884"}, 2025, "--"},
		{[]string{"--data=--", "884"}, []string{"884"}, 2024, "--"},
		{[]string{"-year", "2025", "884"}, []string{"884"}, 2025, "data"},
		{[]string{"-", "884"}, []string{"-", "884"}, 2024, "data"},
		{[]string{}, nil, 2024, "data"},
	}
	for _, c := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		year, data := commonFlags(fs)
		pos, err := parseInterspersed(fs, c.args)
		if err != nil {
			t.Errorf("%v: %v", c.args, err)
			continue
		}
		if !reflect.DeepEqual(pos, c.wantPos) || *year != c.wantYear || *data != c.wantData {
			t.Errorf("%v: pos=%v year=%d data=%q", c.args, pos, *year, *data)
		}
	}
}

func TestParseInterspersedBoolFlag(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	v := fs.Bool("v", false, "")
	year, _ := commonFlags(fs)
	pos, err := parseInterspersed(fs, []string{"-v", "884", "--year", "2025"})
	if err != nil {
		t.Fatal(err)
	}
	if !*v || *year != 2025 || !reflect.DeepEqual(pos, []string{"884"}) {
		t.Fatalf("v=%v year=%d pos=%v", *v, *year, pos)
	}
}

func TestParseInterspersedMissingValue(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	commonFlags(fs)
	if _, err := parseInterspersed(fs, []string{"884", "--year"}); err == nil {
		t.Fatal("expected error for flag without value")
	}
}

func TestParseInterspersedUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	commonFlags(fs)
	if _, err := parseInterspersed(fs, []string{"884", "--nope"}); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseYears(t *testing.T) {
	if ys, err := parseYears("", 2024); err != nil || !reflect.DeepEqual(ys, []int{2024}) {
		t.Errorf("default: %v %v", ys, err)
	}
	if ys, err := parseYears("2025, 2024", 2024); err != nil || !reflect.DeepEqual(ys, []int{2024, 2025}) {
		t.Errorf("sorted: %v %v", ys, err)
	}
	for _, bad := range []string{"2024,2024", "abc", "1999", ","} {
		if _, err := parseYears(bad, 2024); err == nil {
			t.Errorf("expected error for %q", bad)
		}
	}
}

func TestVersionString(t *testing.T) {
	defer func(v string) { version = v }(version)
	version = "v9.9.9"
	if got := versionString(); got != "v9.9.9" {
		t.Errorf("versionString() = %q, want the -X value", got)
	}
	version = ""
	if got := versionString(); got == "" {
		t.Error("versionString() is empty without -X")
	}
}

func copyTestdata(t *testing.T, dst string, year int, edit func(name, body string) string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "testdata", strconv.Itoa(year), "*.csv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		body := string(b)
		if edit != nil {
			body = edit(filepath.Base(p), body)
		}
		if err := os.WriteFile(filepath.Join(dst, filepath.Base(p)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckHeaders(t *testing.T) {
	dir := t.TempDir()
	copyTestdata(t, dir, 2025, nil)
	copyTestdata(t, dir, 2026, nil)
	var out strings.Builder
	if err := checkHeaders(&out, dir, 2026); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ヘッダの比較（2025 → 2026）: 15 表中 4 表に差", "削除 当初予算（合計）", "8 表すべてそろっている"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
	// 前年度がなければ比較は出さず、必須列だけ確かめる
	out.Reset()
	if err := checkHeaders(&out, dir, 2025); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "ヘッダの比較") {
		t.Errorf("no previous year, but compared:\n%s", out.String())
	}
}

func TestCheckHeadersMissingRequired(t *testing.T) {
	dir := t.TempDir()
	copyTestdata(t, dir, 2026, func(name, body string) string {
		if strings.HasPrefix(name, "4-1_") {
			return strings.Replace(body, "所見を踏まえた改善点／概算要求における反映状況,", "反映状況X,", 1)
		}
		return body
	})
	err := checkHeaders(io.Discard, dir, 2026)
	if err == nil || !strings.Contains(err.Error(), "4-1 所見を踏まえた改善点／概算要求における反映状況") {
		t.Fatalf("err = %v", err)
	}
}
