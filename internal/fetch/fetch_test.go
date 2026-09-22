package fetch

import (
	"archive/zip"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZipURL(t *testing.T) {
	got := ZipURL(DefaultBaseURL, 2024, "1-3_RS_2024_基本情報_政策・施策、法令等")
	want := "https://rssystem.go.jp/files/2024/rs/1-3_RS_2024_%E5%9F%BA%E6%9C%AC%E6%83%85%E5%A0%B1_%E6%94%BF%E7%AD%96%E3%83%BB%E6%96%BD%E7%AD%96%E3%80%81%E6%B3%95%E4%BB%A4%E7%AD%89.zip"
	if got != want {
		t.Fatalf("ZipURL:\n got %s\nwant %s", got, want)
	}
}

func TestFileNames(t *testing.T) {
	names := FileNames(2025)
	if len(names) != 15 {
		t.Fatalf("len = %d, want 15", len(names))
	}
	if names[5] != "2-1_RS_2025_予算・執行_サマリ" {
		t.Fatalf("names[5] = %q", names[5])
	}
}

func makeZip(t *testing.T, csvName, body string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("nested/" + csvName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestFetchSkipsExistingAndUnzips(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if !strings.HasPrefix(r.URL.Path, "/2024/rs/") || !strings.HasSuffix(r.URL.Path, ".zip") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
		w.Write(makeZip(t, name+".csv", "\xEF\xBB\xBFa,b\n1,2\n"))
	}))
	defer srv.Close()

	raw := t.TempDir()
	csvDir := t.TempDir()
	// 1 本だけ既存にしておく
	pre := FileNames(2024)[0]
	if err := os.WriteFile(filepath.Join(raw, pre+".zip"), makeZip(t, pre+".csv", "x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Fetch(context.Background(), 2024, raw, csvDir, Options{BaseURL: srv.URL, Delay: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 15 {
		t.Fatalf("results = %d", len(res))
	}
	if !res[0].Skipped || res[1].Skipped {
		t.Fatalf("skip flags wrong: %+v %+v", res[0], res[1])
	}
	if hits != 14 {
		t.Fatalf("server hits = %d, want 14", hits)
	}
	if _, err := os.Stat(filepath.Join(csvDir, FileNames(2024)[1]+".csv")); err != nil {
		t.Fatalf("csv not extracted: %v", err)
	}
}

func TestDownloadRejectsNonZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html>not a zip</html>"))
	}))
	defer srv.Close()
	raw := t.TempDir()
	_, err := Fetch(context.Background(), 2024, raw, t.TempDir(), Options{BaseURL: srv.URL, Delay: 1})
	if err == nil {
		t.Fatal("expected error for non-zip body")
	}
	entries, _ := os.ReadDir(raw)
	if len(entries) != 0 {
		t.Fatalf("raw dir should be empty, got %v", entries)
	}
}

func TestFetchReplacesInvalidCachedZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
		w.Write(makeZip(t, name+".csv", "ok\n"))
	}))
	defer srv.Close()
	raw := t.TempDir()
	first := FileNames(2024)[0]
	bad := filepath.Join(raw, first+".zip")
	if err := os.WriteFile(bad, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Fetch(context.Background(), 2024, raw, t.TempDir(), Options{BaseURL: srv.URL, Delay: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Skipped {
		t.Fatal("invalid cached zip must be re-fetched")
	}
	if _, err := os.Stat(bad + ".bad"); err != nil {
		t.Fatalf("invalid zip should be moved aside: %v", err)
	}
	if err := validateZip(bad); err != nil {
		t.Fatalf("re-fetched zip invalid: %v", err)
	}
}

// makeCRCCorruptZip は CRC が一致しない ZIP を作る（ヘッダは正常）。
func makeCRCCorruptZip(t *testing.T, csvName string) []byte {
	t.Helper()
	good := makeZip(t, csvName, "hello\n")
	corrupt := append([]byte(nil), good...)
	idx := bytes.Index(corrupt, []byte("hello"))
	corrupt[idx] = 'H'
	return corrupt
}

func TestFetchReplacesCRCCorruptCachedZip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
		w.Write(makeZip(t, name+".csv", "ok\n"))
	}))
	defer srv.Close()
	raw := t.TempDir()
	first := FileNames(2024)[0]
	bad := filepath.Join(raw, first+".zip")
	if err := os.WriteFile(bad, makeCRCCorruptZip(t, first+".csv"), 0o644); err != nil {
		t.Fatal(err)
	}
	csvDir := t.TempDir()
	res, err := Fetch(context.Background(), 2024, raw, csvDir, Options{BaseURL: srv.URL, Delay: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res[0].Skipped {
		t.Fatal("CRC-corrupt cached zip must be re-fetched")
	}
	if _, err := os.Stat(bad + ".bad"); err != nil {
		t.Fatalf("corrupt zip should be moved aside: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(csvDir, first+".csv"))
	if err != nil || string(b) != "ok\n" {
		t.Fatalf("csv not replaced from re-fetched zip: %q %v", b, err)
	}
}

func TestFetchRemovesStaleCSVs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimSuffix(filepath.Base(r.URL.Path), ".zip")
		w.Write(makeZip(t, name+".csv", "ok\n"))
	}))
	defer srv.Close()
	csvDir := t.TempDir()
	// 旧版が残した文字化け名と、別年度の CSV（消してはいけない）
	stale := filepath.Join(csvDir, "2-1_RS_2024_\xe4\xba\x88\xe7\xae\x97.csv")
	other := filepath.Join(csvDir, "2-1_RS_2023_予算・執行_サマリ.csv")
	for _, p := range []string{stale, other} {
		if err := os.WriteFile(p, []byte("old\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Fetch(context.Background(), 2024, t.TempDir(), csvDir, Options{BaseURL: srv.URL, Delay: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); err == nil {
		t.Errorf("stale csv should be removed")
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("other-year csv must be kept: %v", err)
	}
	matches, _ := filepath.Glob(filepath.Join(csvDir, "2-1_RS_2024_*.csv"))
	if len(matches) != 1 {
		t.Errorf("expected exactly one 2-1 csv, got %v", matches)
	}
}

func TestUnzipNamesCSVAfterZip(t *testing.T) {
	dir := t.TempDir()
	// エントリ名が壊れていても（Shift_JIS のまま等）、ZIP 名から CSV 名を決める
	zp := filepath.Join(dir, "2-1_RS_2025_予算・執行_サマリ.zip")
	if err := os.WriteFile(zp, makeZip(t, "\x97\x5c\x8e\x5a.csv", "a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Unzip(zp, dir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "2-1_RS_2025_予算・執行_サマリ.csv" {
		t.Fatalf("got %s", got)
	}
}

func TestUnzipKeepsExistingCSVOnFailure(t *testing.T) {
	dir := t.TempDir()
	corrupt := makeCRCCorruptZip(t, "x.csv")
	zp := filepath.Join(dir, "x.zip")
	if err := os.WriteFile(zp, corrupt, 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "x.csv")
	if err := os.WriteFile(dst, []byte("previous\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Unzip(zp, dir); err == nil {
		t.Fatal("expected CRC error")
	}
	b, _ := os.ReadFile(dst)
	if string(b) != "previous\n" {
		t.Fatalf("existing csv was clobbered: %q", b)
	}
	if _, err := os.Stat(dst + ".part"); err == nil {
		t.Fatal("temp file left behind")
	}
}
