package location

// The rows a location is rendered as, in the order they are shown.
//
// One table rather than a sequence of calls, because three things have to agree
// about what the rows are: the standalone page renders them, the panel renders
// them again in TypeScript, and a reader's stored view preferences name them.
// A row that existed in two of those and not the third is the failure this
// table exists to make impossible; `webapp_sync_test.go` holds the TypeScript
// half to the same ids.

// RowID names a row in a way that survives its label being reworded.
//
// These reach the KV store, in a reader's hidden-row list, so they are a
// contract rather than an implementation detail: renaming one silently unhides
// a row for everybody who had hidden it. Add and retire freely; rename never.
type RowID = string

const (
	RowMGRS       RowID = "mgrs"
	RowDecimal    RowID = "decimal"
	RowDMS        RowID = "dms"
	RowDDM        RowID = "ddm"
	RowUSMTF      RowID = "usmtf"
	RowUTM        RowID = "utm"
	RowGEOREF     RowID = "georef"
	RowGARS       RowID = "gars"
	RowPlusCode   RowID = "pluscode"
	RowResolution RowID = "resolution"
	RowConfidence RowID = "confidence"
	RowDatum      RowID = "datum"
	RowRaw        RowID = "raw"
	RowCanonical  RowID = "canonical"
)

// Row is one line of the rendered location.
type Row struct {
	ID    RowID
	Label string

	// Copyable is false for the rows that are prose rather than a value. A
	// reader pasting "about 11 m" or "WGS 84" into another system has copied a
	// sentence, not a position.
	Copyable bool

	// Value renders the row, or "" for a row that does not apply.
	//
	// Empty means OMIT rather than "render a blank", and every conditional row
	// is expressed that way: the grid rows are empty at the poles, where the
	// notation is Universal Polar Stereographic and this package does not
	// reach; Confidence is empty unless the token stated one; and Normalized is
	// empty when it would only repeat the author's own text.
	Value func(l Location, raw string) string
}

// Rows is every row, in render order.
var Rows = []Row{
	{RowMGRS, "MGRS", true, func(l Location, _ string) string { return l.MGRSText() }},
	{RowDecimal, "Lat / lon", true, func(l Location, _ string) string { return l.DecimalText() }},
	{RowDMS, "DMS", true, func(l Location, _ string) string { return l.DMSText() }},
	{RowDDM, "DDM", true, func(l Location, _ string) string { return l.DDMText() }},
	{RowUSMTF, "USMTF", true, func(l Location, _ string) string { return l.USMTFText() }},
	{RowUTM, "UTM", true, func(l Location, _ string) string { return l.UTMText() }},
	{RowGEOREF, "GEOREF", true, func(l Location, _ string) string { return l.GEOREFText() }},
	{RowGARS, "GARS", true, func(l Location, _ string) string { return l.GARSText() }},
	{RowPlusCode, "Plus Code", true, func(l Location, _ string) string { return l.PlusCodeText() }},
	{RowResolution, "Resolution", false, func(l Location, _ string) string { return l.ResolutionText() }},
	{RowConfidence, "Confidence", false, func(l Location, _ string) string { return l.ConfidenceText() }},
	{RowDatum, "Datum", false, func(Location, string) string { return "WGS 84" }},
	{RowRaw, "Original text", true, func(_ Location, raw string) string { return raw }},

	// Only when it says something the row above did not. For the whole USMTF
	// compact family the author's text already is the canonical form.
	{RowCanonical, "Normalized", true, func(l Location, raw string) string {
		if canonical := l.Canonical(); canonical != raw {
			return canonical
		}
		return ""
	}},
}

// AllRowIDs is every row id.
//
// For tests that want to drive something over the whole catalog. Production
// code asks Rows directly: this existed only to be measured with len(), which
// len(Rows) already answers.
var AllRowIDs = func() []RowID {
	ids := make([]RowID, 0, len(Rows))
	for _, row := range Rows {
		ids = append(ids, row.ID)
	}
	return ids
}()

var rowByID = func() map[RowID]bool {
	m := make(map[RowID]bool, len(Rows))
	for _, row := range Rows {
		m[row.ID] = true
	}
	return m
}()

// KnownRow reports whether an id names a row this build renders.
func KnownRow(id RowID) bool { return rowByID[id] }
