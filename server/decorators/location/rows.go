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
	RowRegion     RowID = "region"
	RowResolution RowID = "resolution"
	RowConfidence RowID = "confidence"
	RowDatum      RowID = "datum"
	RowRaw        RowID = "raw"
	RowCanonical  RowID = "canonical"
)

// Row is one line of the rendered location.
//
// A catalog rather than a renderer. It carried a Value closure per row while the
// page was rendered in Go; every surface renders from the webapp's format.ts
// now, so what is left here is the contract three things have to agree on: the
// ids a reader's hidden-row list names, the labels, and which rows are worth
// copying. webapp_sync_test.go holds the TypeScript half to all three.
type Row struct {
	ID    RowID
	Label string

	// Copyable is false for the rows that are prose rather than a value. A
	// reader pasting "about 11 m" or "WGS 84" into another system has copied a
	// sentence, not a position.
	Copyable bool
}

// Rows is every row, in render order.
var Rows = []Row{
	{RowDecimal, "Lat / lon", true},
	{RowDMS, "DMS", true},
	{RowDDM, "DDM", true},
	{RowMGRS, "MGRS", true},
	{RowUSMTF, "USMTF", true},
	{RowUTM, "UTM", true},
	{RowRegion, "Region", false},
	{RowResolution, "Resolution", false},
	{RowConfidence, "Confidence", false},
	{RowDatum, "Datum", false},
	{RowRaw, "Original text", true},
	{RowCanonical, "Normalized", true},
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

const SectionMap RowID = "map"

func KnownRow(id RowID) bool { return rowByID[id] || id == SectionMap }
