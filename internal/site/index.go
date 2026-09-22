package site

import (
	"bytes"
	"encoding/json"
	"html/template"
	"io"
	"math"
	"sort"

	"github.com/kwrkb/zailoop/internal/lifecycle"
	"github.com/kwrkb/zailoop/internal/render"
)

// Row は一覧 JSON の 1 事業分。キーは短く、無効な値は省略する。
type Row struct {
	ID         string    `json:"id"`
	Name       string    `json:"n"`
	Ministry   int       `json:"m"`            // Meta.Ministries の添字
	Category   int       `json:"c"`            // Meta.Categories の添字
	Reflection int       `json:"rf"`           // Meta.Reflections の添字（-1 なし）
	Request    *int64    `json:"rq,omitempty"` // ① 概算要求
	Initial    *int64    `json:"in,omitempty"` // ② 当初
	Current    *int64    `json:"cu,omitempty"` // ② 現額
	Executed   *int64    `json:"ex,omitempty"` // ③ 執行
	ExecRate   *float64  `json:"er,omitempty"` // ③ 執行率
	ExecState  string    `json:"xs,omitempty"` // ③ 状態（OK 以外）
	CarriedOut *int64    `json:"co,omitempty"` // ④ 翌年度繰越
	Unused     *int64    `json:"un,omitempty"` // ④ 不用相当額
	UnusedDiff *int64    `json:"ud,omitempty"` // ④ 負の差額
	UnusedSt   string    `json:"us,omitempty"` // ④ 状態（OK 以外）
	Reflected  *int64    `json:"ra,omitempty"` // ⑥ 反映額
	NextInit   *int64    `json:"ni,omitempty"` // FY N+1 当初
	NextReq    *int64    `json:"nr,omitempty"` // ⑥ FY N+2 要求
	ByYear     []*int64  `json:"yr"`           // Meta.Years と同順の当初予算
	Outcomes   int       `json:"oc,omitempty"` // アウトカム指標数
	Rates      []float64 `json:"or,omitempty"` // 達成率（%）
	Signals    uint16    `json:"sg,omitempty"` // Signal ビット
}

// SignalMeta は Signal の説明と件数。
type SignalMeta struct {
	Bit         uint16 `json:"bit"`
	Code        string `json:"code"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Count       int    `json:"count"`
}

// Meta は一覧 JSON の付帯情報。
type Meta struct {
	SheetYear   int                  `json:"sheetYear"`
	ActualYear  int                  `json:"actualYear"`
	Years       []int                `json:"years"` // Row.ByYear の年度
	Ministries  []string             `json:"ministries"`
	Categories  []string             `json:"categories"`
	Reflections []string             `json:"reflections"`
	Signals     []SignalMeta         `json:"signals"`
	Thresholds  lifecycle.Thresholds `json:"thresholds"`
	Count       int                  `json:"count"`
	Attribution string               `json:"attribution"`
	Generated   string               `json:"generated"`
}

// Payload は index.html に埋め込む JSON 全体。
type Payload struct {
	Meta Meta  `json:"meta"`
	Rows []Row `json:"rows"`
}

// indexBuilder は Summary を Row に変換しつつ meta の辞書を育てる。
type indexBuilder struct {
	years       []int
	ministries  dict
	categories  dict
	reflections dict
	rows        []Row
	counts      map[lifecycle.Signal]int
}

type dict struct {
	idx  map[string]int
	list []string
}

func (d *dict) get(s string) int {
	if d.idx == nil {
		d.idx = map[string]int{}
	}
	if i, ok := d.idx[s]; ok {
		return i
	}
	d.idx[s] = len(d.list)
	d.list = append(d.list, s)
	return len(d.list) - 1
}

func newIndexBuilder(sheetYear int) *indexBuilder {
	b := &indexBuilder{counts: map[lifecycle.Signal]int{}}
	for y := sheetYear - 3; y <= sheetYear; y++ {
		b.years = append(b.years, y)
	}
	return b
}

func (b *indexBuilder) add(sm lifecycle.Summary) {
	r := Row{
		ID: sm.ID, Name: sm.Name,
		Ministry: b.ministries.get(sm.Ministry), Category: b.categories.get(sm.Category), Reflection: -1,
		Request: yenPtr(sm.Request), Initial: yenPtr(sm.Initial), Current: yenPtr(sm.Current), Executed: yenPtr(sm.Executed),
		CarriedOut: yenPtr(sm.CarriedOut), Unused: yenPtr(sm.Unused), Reflected: yenPtr(sm.Reflected),
		NextInit: yenPtr(sm.NextInitial), NextReq: yenPtr(sm.NextRequest),
		Outcomes: sm.OutcomeCount, Rates: sm.OutcomeRates, Signals: uint16(sm.Signals),
	}
	if sm.Reflection != "" {
		r.Reflection = b.reflections.get(sm.Reflection)
	}
	if sm.ExecRate.Valid {
		v := math.Round(sm.ExecRate.Value*1000) / 1000
		r.ExecRate = &v
	}
	if sm.ExecState != lifecycle.StateOK {
		r.ExecState = sm.ExecState.String()
	}
	if sm.UnusedState != lifecycle.StateOK {
		r.UnusedSt = sm.UnusedState.String()
		if sm.UnusedState == lifecycle.StateNeedsReview {
			d := sm.UnusedDiff
			r.UnusedDiff = &d
		}
	}
	r.ByYear = make([]*int64, len(b.years))
	for i, y := range sm.Years {
		for j, yy := range b.years {
			if y == yy {
				r.ByYear[j] = yenPtr(sm.InitialByYear[i])
			}
		}
	}
	for _, d := range lifecycle.SignalInfo {
		if sm.Signals.Has(d.Signal) {
			b.counts[d.Signal]++
		}
	}
	b.rows = append(b.rows, r)
}

func (b *indexBuilder) payload(sheetYear, actualYear int, th lifecycle.Thresholds, generated string) Payload {
	var sigs []SignalMeta
	for _, d := range lifecycle.SignalInfo {
		sigs = append(sigs, SignalMeta{Bit: uint16(d.Signal), Code: d.Code, Label: d.Label, Description: d.Description, Count: b.counts[d.Signal]})
	}
	return Payload{
		Meta: Meta{
			SheetYear: sheetYear, ActualYear: actualYear, Years: b.years,
			Ministries: nonNil(b.ministries.list), Categories: nonNil(b.categories.list), Reflections: nonNil(b.reflections.list),
			Signals: sigs, Thresholds: th, Count: len(b.rows), Attribution: render.Attribution, Generated: generated,
		},
		Rows: b.rows,
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// indexData は index.html に渡すデータ。
type indexData struct {
	JSON        template.JS
	Meta        Meta
	Attribution string
}

// writeIndex は一覧ページを書く。JSON は encoding/json の既定エスケープ（< > & を \u 化）で
// </script> の混入を防ぎ、template.JS で html/template の再エスケープを避ける。
func writeIndex(w io.Writer, p Payload) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(p); err != nil {
		return err
	}
	return templates.ExecuteTemplate(w, "index.html", indexData{JSON: template.JS(bytes.TrimSpace(buf.Bytes())), Meta: p.Meta, Attribution: render.Attribution})
}

// listGroup は list.html の府省庁ごとのまとまり。
type listGroup struct {
	Ministry string
	Rows     []Row
}

type listData struct {
	Meta        Meta
	Groups      []listGroup
	Attribution string
}

// writeList は JS なしで全事業へ辿れる一覧を書く。
func writeList(w io.Writer, p Payload) error {
	byM := map[int][]Row{}
	for _, r := range p.Rows {
		byM[r.Ministry] = append(byM[r.Ministry], r)
	}
	var groups []listGroup
	for i, m := range p.Meta.Ministries {
		rows := byM[i]
		sort.SliceStable(rows, func(a, b int) bool { return rows[a].ID < rows[b].ID })
		groups = append(groups, listGroup{Ministry: m, Rows: rows})
	}
	return templates.ExecuteTemplate(w, "list.html", listData{Meta: p.Meta, Groups: groups, Attribution: render.Attribution})
}
