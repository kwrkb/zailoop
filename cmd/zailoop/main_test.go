package main

import (
	"flag"
	"io"
	"reflect"
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
