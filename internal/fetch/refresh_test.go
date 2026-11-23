package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// distServer は配布元の代わり。body(name) の内容を ZIP にして返す。fail に含む名前は 500。
type distServer struct {
	t    *testing.T
	mu   sync.Mutex
	hits int
	body func(name string) string
	fail map[string]bool
}

func (d *distServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	d.hits++
	d.mu.Unlock()
	name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
	if d.fail[name] {
		http.Error(w, "boom", http.StatusInternalServerError)
		return
	}
	w.Write(makeZip(d.t, name+".csv", d.body(name)))
}

func fixedClock(s string) func() time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return func() time.Time { return tm }
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFetchRecordsHashAndDate(t *testing.T) {
	d := &distServer{t: t, body: func(string) string { return "v1\n" }}
	srv := httptest.NewServer(d)
	defer srv.Close()
	raw, csvDir := t.TempDir(), t.TempDir()
	res, err := Fetch(context.Background(), 2024, raw, csvDir, Options{BaseURL: srv.URL, Delay: 1, Now: fixedClock("2026-11-23T10:00:00+09:00")})
	if err != nil {
		t.Fatal(err)
	}
	m, err := loadManifest(raw, 2024)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 15 {
		t.Fatalf("entries = %d", len(m.Files))
	}
	for i, e := range m.Files {
		if e.Name != FileNames(2024)[i] {
			t.Fatalf("entry %d = %s, want FileNames order", i, e.Name)
		}
		if e.FetchedAt != "2026-11-23T10:00:00+09:00" || e.CheckedAt != e.FetchedAt {
			t.Fatalf("dates: %+v", e)
		}
		zs, _ := fileSHA256(filepath.Join(raw, e.Name+".zip"))
		cs, _ := fileSHA256(filepath.Join(csvDir, e.Name+".csv"))
		if e.ZipSHA256 != zs || e.CSVSHA256 != cs || res[i].ZipSHA != zs {
			t.Fatalf("hash mismatch: %+v", e)
		}
	}
}

func TestFetchBackfillsRecordWithoutDate(t *testing.T) {
	d := &distServer{t: t, body: func(string) string { return "v1\n" }}
	srv := httptest.NewServer(d)
	defer srv.Close()
	raw := t.TempDir()
	// 記録より前に取得した ZIP が 15 本ある
	for _, n := range FileNames(2024) {
		if err := os.WriteFile(filepath.Join(raw, n+".zip"), makeZip(t, n+".csv", "old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Fetch(context.Background(), 2024, raw, t.TempDir(), Options{BaseURL: srv.URL, Delay: 1}); err != nil {
		t.Fatal(err)
	}
	if d.hits != 0 {
		t.Fatalf("hits = %d, want 0", d.hits)
	}
	m, _ := loadManifest(raw, 2024)
	for _, e := range m.Files {
		if e.ZipSHA256 == "" || e.CSVSHA256 == "" {
			t.Fatalf("hash not backfilled: %+v", e)
		}
		if e.FetchedAt != "" || e.CheckedAt != "" {
			t.Fatalf("unknown fetch date must stay empty: %+v", e)
		}
	}
}

func TestRefreshUnchanged(t *testing.T) {
	d := &distServer{t: t, body: func(string) string { return "v1\n" }}
	srv := httptest.NewServer(d)
	defer srv.Close()
	raw, csvDir := t.TempDir(), t.TempDir()
	opt := Options{BaseURL: srv.URL, Delay: 1, Now: fixedClock("2026-11-01T00:00:00Z")}
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err != nil {
		t.Fatal(err)
	}
	d.hits = 0
	opt.Refresh = true
	opt.Now = fixedClock("2026-11-23T00:00:00Z")
	res, err := Fetch(context.Background(), 2024, raw, csvDir, opt)
	if err != nil {
		t.Fatal(err)
	}
	if d.hits != 15 {
		t.Fatalf("hits = %d, want 15", d.hits)
	}
	for _, r := range res {
		if r.Changed || r.Skipped {
			t.Fatalf("unexpected state: %+v", r)
		}
	}
	if _, err := os.Stat(filepath.Join(raw, "old")); err == nil {
		t.Fatal("nothing should be moved to old/")
	}
	m, _ := loadManifest(raw, 2024)
	for _, e := range m.Files {
		if e.FetchedAt != "2026-11-01T00:00:00Z" || e.CheckedAt != "2026-11-23T00:00:00Z" {
			t.Fatalf("dates: %+v", e)
		}
	}
	if parts, _ := filepath.Glob(filepath.Join(raw, "*.part")); len(parts) != 0 {
		t.Fatalf("temp files left: %v", parts)
	}
}

func TestRefreshReplacesChangedAndKeepsOld(t *testing.T) {
	version := "v1\n"
	target := FileNames(2024)[5]
	d := &distServer{t: t, body: func(name string) string {
		if name == target {
			return version
		}
		return "same\n"
	}}
	srv := httptest.NewServer(d)
	defer srv.Close()
	raw, csvDir := t.TempDir(), t.TempDir()
	opt := Options{BaseURL: srv.URL, Delay: 1, Now: fixedClock("2026-11-01T00:00:00Z")}
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err != nil {
		t.Fatal(err)
	}
	m0, _ := loadManifest(raw, 2024)
	oldSHA := m0.Files[5].ZipSHA256

	version = "v2\n"
	opt.Refresh = true
	opt.Now = fixedClock("2026-11-23T00:00:00Z")
	res, err := Fetch(context.Background(), 2024, raw, csvDir, opt)
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range res {
		if r.Changed != (i == 5) || r.CSVChanged != (i == 5) {
			t.Fatalf("Changed[%d] = %v, CSVChanged = %v", i, r.Changed, r.CSVChanged)
		}
	}
	prev := filepath.Join(raw, "old", target+"."+oldSHA[:12]+".zip")
	if res[5].Previous != prev {
		t.Fatalf("Previous = %s, want %s", res[5].Previous, prev)
	}
	if s, _ := fileSHA256(prev); s != oldSHA {
		t.Fatal("old zip not preserved")
	}
	if got := readFile(t, filepath.Join(csvDir, target+".csv")); got != "v2\n" {
		t.Fatalf("csv = %q", got)
	}
	m, _ := loadManifest(raw, 2024)
	if e := m.Files[5]; e.FetchedAt != "2026-11-23T00:00:00Z" || e.ZipSHA256 == oldSHA {
		t.Fatalf("changed entry: %+v", e)
	}
	if e := m.Files[0]; e.FetchedAt != "2026-11-01T00:00:00Z" || e.CheckedAt != "2026-11-23T00:00:00Z" {
		t.Fatalf("unchanged entry: %+v", e)
	}
}

func TestRefreshFailureLeavesEverythingIntact(t *testing.T) {
	version := "v1\n"
	d := &distServer{t: t, body: func(string) string { return version }}
	srv := httptest.NewServer(d)
	defer srv.Close()
	raw, csvDir := t.TempDir(), t.TempDir()
	opt := Options{BaseURL: srv.URL, Delay: 1, Now: fixedClock("2026-11-01T00:00:00Z")}
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, manifestPath(raw, 2024))

	version = "v2\n"
	d.fail = map[string]bool{FileNames(2024)[10]: true}
	opt.Refresh = true
	opt.Now = fixedClock("2026-11-23T00:00:00Z")
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err == nil {
		t.Fatal("expected error")
	}
	if after := readFile(t, manifestPath(raw, 2024)); after != before {
		t.Fatalf("manifest changed:\n%s", after)
	}
	m, _ := loadManifest(raw, 2024)
	for _, e := range m.Files {
		if got := readFile(t, filepath.Join(csvDir, e.Name+".csv")); got != "v1\n" {
			t.Fatalf("%s csv = %q, want v1", e.Name, got)
		}
		if s, _ := fileSHA256(filepath.Join(raw, e.Name+".zip")); s != e.ZipSHA256 {
			t.Fatalf("%s zip was replaced", e.Name)
		}
	}
	if parts, _ := filepath.Glob(filepath.Join(raw, "*.part")); len(parts) != 0 {
		t.Fatalf("temp files left: %v", parts)
	}
	if _, err := os.Stat(filepath.Join(raw, "old")); err == nil {
		t.Fatal("nothing should be moved to old/")
	}
}

func TestRefreshDistinguishesRepackFromCSVChange(t *testing.T) {
	entry := "a.csv"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CSV の本文は同じで、ZIP のエントリ名だけが変わる（詰め直し）
		w.Write(makeZip(t, entry, "same\n"))
	}))
	defer srv.Close()
	raw, csvDir := t.TempDir(), t.TempDir()
	opt := Options{BaseURL: srv.URL, Delay: 1}
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err != nil {
		t.Fatal(err)
	}
	entry = "b.csv"
	var log strings.Builder
	opt.Refresh, opt.Log = true, &log
	res, err := Fetch(context.Background(), 2024, raw, csvDir, opt)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if !r.Changed || r.CSVChanged {
			t.Fatalf("want repacked (zip changed, csv same): %+v", r)
		}
	}
	if n := strings.Count(log.String(), "repacked "); n != 15 {
		t.Fatalf("repacked lines = %d\n%s", n, log.String())
	}
}

func TestRefreshLogsStates(t *testing.T) {
	version := "v1\n"
	target := FileNames(2024)[3]
	d := &distServer{t: t, body: func(name string) string {
		if name == target {
			return version
		}
		return "same\n"
	}}
	srv := httptest.NewServer(d)
	defer srv.Close()
	raw, csvDir := t.TempDir(), t.TempDir()
	opt := Options{BaseURL: srv.URL, Delay: 1}
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err != nil {
		t.Fatal(err)
	}
	// 1 本は手元から消えている（--refresh で新規に取得する）
	missing := FileNames(2024)[0]
	if err := os.Remove(filepath.Join(raw, missing+".zip")); err != nil {
		t.Fatal(err)
	}
	version = "v2\n"
	var log strings.Builder
	opt.Refresh, opt.Log = true, &log
	if _, err := Fetch(context.Background(), 2024, raw, csvDir, opt); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{missing: "fetched", target: "updated", FileNames(2024)[1]: "unchanged"}
	seen := 0
	for _, line := range strings.Split(strings.TrimSpace(log.String()), "\n") {
		f := strings.Fields(line)
		if w, ok := want[f[len(f)-1]]; ok {
			seen++
			if f[0] != w {
				t.Errorf("%s: state %s, want %s", f[len(f)-1], f[0], w)
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("seen %d of %d files in log:\n%s", seen, len(want), log.String())
	}
}
