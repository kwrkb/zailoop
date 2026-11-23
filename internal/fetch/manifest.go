package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// manifest は 1 年度分の取得記録。rawDir/fetch-<year>.json に置く。
type manifest struct {
	Year  int      `json:"year"`
	Files []*entry `json:"files"`
}

// entry は 1 ファイルの取得記録。
type entry struct {
	Name      string `json:"name"`
	ZipSHA256 string `json:"zip_sha256"`
	CSVSHA256 string `json:"csv_sha256"`
	// FetchedAt はこの内容の ZIP を取得した日時（RFC 3339）。記録より前に取得した
	// ZIP など、いつ取得したか分からないときは空。
	FetchedAt string `json:"fetched_at"`
	// CheckedAt は配布元の内容がこの ZIP と同じだと最後に確かめた日時（取得時または --refresh 時）。
	CheckedAt string `json:"checked_at"`

	previous string // refresh で退避した旧 ZIP（記録しない）
}

func manifestPath(rawDir string, year int) string {
	return filepath.Join(rawDir, fmt.Sprintf("fetch-%d.json", year))
}

func loadManifest(rawDir string, year int) (*manifest, error) {
	m := &manifest{Year: year}
	b, err := os.ReadFile(manifestPath(rawDir, year))
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, m); err != nil {
		return nil, fmt.Errorf("%s: %w", manifestPath(rawDir, year), err)
	}
	if m.Year != year {
		return nil, fmt.Errorf("%s: 年度が %d になっている", manifestPath(rawDir, year), m.Year)
	}
	return m, nil
}

// entry は name の記録を返す。なければ FileNames の順に並ぶ位置へ足す。
func (m *manifest) entry(name string) *entry {
	for _, e := range m.Files {
		if e.Name == name {
			return e
		}
	}
	e := &entry{Name: name}
	m.Files = append(m.Files, e)
	order := map[string]int{}
	for i, n := range FileNames(m.Year) {
		order[n] = i
	}
	for i := len(m.Files) - 1; i > 0 && order[m.Files[i].Name] < order[m.Files[i-1].Name]; i-- {
		m.Files[i], m.Files[i-1] = m.Files[i-1], m.Files[i]
	}
	return e
}

// save は一時ファイルに書いてから置き換える。記録するファイルがなければ書かない。
func (m *manifest) save(rawDir string) error {
	if len(m.Files) == 0 {
		return nil
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	dst := manifestPath(rawDir, m.Year)
	tmp := dst + ".part"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
