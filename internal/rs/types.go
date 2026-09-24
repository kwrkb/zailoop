// Package rs は行政事業レビュー見える化サイトが配布するレビューシート CSV を読み、
// 事業単位の構造体に組み立てる。I/O と列名の知識はこのパッケージに閉じ込め、
// 上位（lifecycle, render）は列名を知らない。
//
// 設計上の約束:
//   - 列は番号ではなくヘッダ名で参照する（docs/data-survey.md §2 のヘッダが正）。
//   - 金額は円単位の int64 に「値の有無」を添える（Yen）。空欄と 0 を区別する。
//   - 2-1 は合計行と会計別行を両方保持し、表示は合計行を基準にする。
//     行の分類は会計区分の有無ではなく「合計列が埋まっているか」で行う。
//   - 数値は TrimSpace してから解析する（5-1 の末尾空白対策）。
//   - 「34482000.0」のような小数点付き整数は浮動小数点を経由せずに整数化する。
package rs

// Yen は円単位の金額。Valid が false なら CSV 上は空欄。
type Yen struct {
	Value int64
	Valid bool
}

// Ratio は執行率・達成率など小数で表される比率。Valid が false なら空欄。
type Ratio struct {
	Value float64
	Valid bool
}

// Project は 1-2「基本情報_事業概要等」の 1 事業。主要経費で重複する行は 1 つに畳む。
type Project struct {
	ID           string   // 予算事業ID
	Name         string   // 事業名
	Ministry     string   // 府省庁
	Bureau       string   // 局・庁
	Division     string   // 課
	Purpose      string   // 事業の目的
	Summary      string   // 事業の概要
	Category     string   // 事業区分（前年度事業/新規開始事業/新規要求事業）
	StartYear    string   // 事業開始年度（空あり）
	EndYear      string   // 事業終了（予定）年度（空あり）
	MajorExpense []string // 主要経費（複数行を集約、重複なし、出現順）
	Methods      []string // 実施方法で "1" が立っている項目名（直接実施/補助/負担/交付/分担金・拠出金/その他）
	URL          string   // 事業概要URL（府省庁が示す事業のページ。空あり）
	Remarks      string   // 備考（シート作成の経緯・過年度の執行額・改訂履歴などの自由記述。空あり）
}

// BudgetTotal は 2-1 の合計行（列「当初予算（合計）」〜「翌年度要求額（合計）」）。
type BudgetTotal struct {
	Initial       Yen    // 当初予算（合計）
	Supplementary Yen    // 補正予算（合計）
	CarriedIn     Yen    // 前年度からの繰越し（合計）
	Reserve       Yen    // 予備費等（合計）
	Current       Yen    // 計（歳出予算現額合計）
	Executed      Yen    // 執行額（合計）
	ExecRate      Ratio  // 執行率
	CarriedOut    Yen    // 翌年度への繰越し(合計）
	NextRequest   Yen    // 翌年度要求額（合計）
	ChangeReason  string // 主な増減理由
	Notes         string // その他特記事項
}

// BudgetAccount は 2-1 の会計別行（列「当初予算」〜「要望額」）。
type BudgetAccount struct {
	Category      string // 会計区分（一般会計/特別会計）
	Account       string // 会計
	Subaccount    string // 勘定
	Initial       Yen    // 当初予算
	Supplementary [5]Yen // 第1次〜第5次補正予算
	CarriedIn     Yen    // 前年度から繰越し
	Reserve       [4]Yen // 予備費等1〜4
	Current       Yen    // 歳出予算現額
	Executed      Yen    // 執行額
	NextRequest   Yen    // 翌年度要求額
	Demand        Yen    // 要望額
	Notes         string // 備考
}

// BudgetYear は 1 事業の 1 予算年度分。Total は公表された合計行、Accounts はその内訳。
// Issues は読み取り時の整合チェックで見つかった警告（合計行と内訳の不一致、合計行の欠落・重複など）。
// 不一致があっても Total は書き換えない。
type BudgetYear struct {
	Year     int // 予算年度
	Total    BudgetTotal
	HasTotal bool // 合計行が 1 行存在したか
	Accounts []BudgetAccount
	Issues   []string
}

// BudgetItem は 2-2「予算種別・歳出予算項目」の 1 行。
type BudgetItem struct {
	Year        int    // 予算年度
	Category    string // 会計区分
	Account     string // 会計
	Subaccount  string // 勘定
	Kind        string // 予算種別（当初予算/第1次補正予算/前年度から繰越し/予備費等1 …）
	Owner       string // 所管
	Org         string // 組織・勘定
	Ko          string // 項
	Moku        string // 目
	Note        string // 歳出予算項目の補足情報
	Amount      Yen    // 予算額（歳出予算項目ごと）
	NextRequest Yen    // 翌年度要求額（歳出予算項目ごと）
}

// Indicator は 3-1「効果発現経路_目標・実績」の 1 指標。
// CSV では 1 指標が「1.目標年度 / 2.目標値 / 3.実績値 / 4.達成率」の 4 行に分かれ、
// 値は 2007〜2060 の年度列に横持ちされている。ここでは年度をキーにした map に畳む。
// 値は数値以外（"-" など）が入りうるため生文字列で保持する。
type Indicator struct {
	Number     string         // アクティビティ・アウトプット・アウトカムの番号
	Kind       string         // 種別（アクティビティ/アウトプット/アウトカム）
	Term       string         // アウトカムの期間（1.短期/2.中期/3.長期。"長期" など揺れあり、正規化しない）
	TargetType string         // 成果目標の種類（定量的/定性的）
	Goal       string         // アクティビティ／活動目標／成果目標
	Metric     string         // 活動指標／成果指標
	Unit       string         // 単位
	Direction  string         // 改善の上向き／下向き
	Source     string         // 根拠として用いた統計・データ名
	TargetYear map[int]string // 目標年度フラグ（値 "1" が立った年度）
	Targets    map[int]string // 目標値（年度→生文字列）
	Actuals    map[int]string // 実績値
	Rates      map[int]string // 達成率
}

// Remark は 4-1 の「過去に受けた指摘事項」「その他の指摘事項」の 1 件。
type Remark struct {
	Kind    string // 区分または調査等の名称
	Year    string // 取りまとめ年度または指摘年度
	Content string // 内容
	Status  string // 対応状況
}

// SpecialReflection は 4-1「反映額（特別会計）」の 1 件。
type SpecialReflection struct {
	Account    string
	Subaccount string
	Amount     Yen
}

// Evaluation は 4-1「点検・評価」の 1 事業。指摘事項で重複する行は 1 つに畳む。
type Evaluation struct {
	SelfCheck            string // 事業所管部局による点検・改善ー点検結果
	Improvement          string // 事業所管部局による点検・改善ー改善の方向性
	TargetYearAssessment string // 〜目標年度における効果測定に関する評価
	ExternalYear         string // 外部有識者による点検ー最終実施年度
	ExternalTarget       string // 外部有識者による点検ー点検対象
	ExternalReason       string // 外部有識者による点検ー対象の理由
	ExternalOpinion      string // 外部有識者による点検ー所見
	PublicProcess        string // 公開プロセス結果概要
	TeamOpinion          string // 行政事業レビュー推進チームの所見
	TeamOpinionDetail    string // 〜の詳細
	Reflection           string // 所見を踏まえた改善点／概算要求における反映状況
	ReflectionDetail     string // 〜の詳細
	ReflectedGeneral     Yen    // 反映額（一般会計）
	ReflectedSpecial     []SpecialReflection
	PastRemarks          []Remark // 過去に受けた指摘事項
	OtherRemarks         []Remark // その他の指摘事項
}

// Contract は 5-1 の契約行（「金額」が入る行）。
type Contract struct {
	Summary      string // 契約概要
	Amount       Yen    // 金額
	Method       string // 契約方式等
	MethodDetail string // 具体的な契約方式等
	Bidders      string // 入札者数
	Rate         string // 落札率
}

// Payee は 5-1 の支出先。合計行（「支出先の合計支出額」が入る行）と契約行を 1 つに畳む。
type Payee struct {
	Name            string // 支出先名
	CorporateNumber string // 法人番号（文字列のまま、TrimSpace 済み）
	Location        string // 所在地
	Kind            string // 法人種別
	Total           Yen    // 支出先の合計支出額
	Contracts       []Contract
}

// PayeeBlock は 5-1 の支出ブロック。ブロック行（「ブロックの合計支出額」が入る行）が見出し。
// ブロック間の資金移動（5-2）があるため、ブロック合計を足して事業総額にしてはならない。
type PayeeBlock struct {
	Number     string // 支出先ブロック番号（A, B, …）
	Name       string // 支出先ブロック名
	PayeeCount string // 支出先の数（生文字列）
	Role       string // 事業を行う上での役割
	Total      Yen    // ブロックの合計支出額
	Payees     []Payee
}

// Obligation は 5-4「国庫債務負担行為等による契約」の 1 件。5-1 の支出とは別の表で、
// 契約額を予算表（2-1）の執行額や 5-1 の支出額と足し合わせてはならない。
type Obligation struct {
	Block   string // 支出先ブロック（5-1 のブロック記号と対応）
	Payee   string // 契約先名
	Summary string // 契約概要（契約名）
	Amount  Yen    // 契約額
	Method  string // 契約方式等
	Bidders string // 入札者数（応募者数）
	Rate    string // 落札率（％）
}

// Related は 1-5「関連事業」の 1 件。関連事業のない事業は空欄 1 行だけを持つので読み飛ばす。
// ID はレビューシートのない事業（基金シート・セグメントシートなど）を指すことがある。
type Related struct {
	ID   string // 関連事業の事業ID
	Name string // 関連事業の事業名
	Kind string // 関連性（親事業/子事業/統合元/統合先/分割元/分割先/基金造成した基金シート …）
}

// Sheet は 1 年度分の CSV 群から組み立てた 1 事業のレビューシート。
type Sheet struct {
	FiscalYear  int // 事業年度（CSV の「事業年度」列）
	Project     Project
	Related     []Related    // 1-5、CSV の出現順
	Budgets     []BudgetYear // 予算年度の昇順
	Items       []BudgetItem // 2-2、CSV の出現順
	Indicators  []Indicator  // 3-1、CSV の出現順
	Evaluation  Evaluation
	Blocks      []PayeeBlock // 5-1、CSV の出現順
	Obligations []Obligation // 5-4、CSV の出現順
}

// Budget は指定した予算年度の BudgetYear を返す。無ければ nil。
func (s *Sheet) Budget(year int) *BudgetYear {
	for i := range s.Budgets {
		if s.Budgets[i].Year == year {
			return &s.Budgets[i]
		}
	}
	return nil
}
