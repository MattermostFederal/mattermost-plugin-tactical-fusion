package cot

// SectionID names a group of the sidebar panel in a way that survives its label
// being reworded.
//
// These reach the KV store, in a reader's hidden-section list, so they are a
// contract rather than an implementation detail: renaming one silently unhides
// a section for everybody who had hidden it. Add and retire freely; rename
// never.
type SectionID = string

const (
	SectionMap         SectionID = "map"
	SectionStale       SectionID = "stale"
	SectionEvent       SectionID = "event"
	SectionRemarks     SectionID = "remarks"
	SectionDevice      SectionID = "device"
	SectionPrecision   SectionID = "precision"
	SectionOrientation SectionID = "orientation"
	SectionPayload     SectionID = "payload"
	SectionShape       SectionID = "shape"
	SectionFlow        SectionID = "flow"
	SectionSource      SectionID = "source"
)

// Section is one hideable group of the sidebar panel.
//
// A catalog and a validator input, nothing else: this package renders none of
// it. Two things have to agree about what the sections are, and this is the one
// that decides which ids the server will store; cot_sync_test.go holds the
// TypeScript half to the same ids in the same order.
type Section struct {
	ID    SectionID
	Label string
}

// Sections is every hideable section, in the order the panel draws them.
var Sections = []Section{
	{SectionMap, "Map"},
	{SectionStale, "Goes stale"},
	{SectionEvent, "Event readings"},
	{SectionRemarks, "Remarks"},
	{SectionDevice, "Device"},
	{SectionPrecision, "Position quality"},
	{SectionOrientation, "Orientation"},
	{SectionPayload, "Payload"},
	{SectionShape, "Shape"},
	{SectionFlow, "Processing path"},
	{SectionSource, "As posted"},
}

var sectionByID = func() map[SectionID]bool {
	m := make(map[SectionID]bool, len(Sections))
	for _, section := range Sections {
		m[section.ID] = true
	}
	return m
}()

// KnownSection reports whether an id names a section this build renders.
func KnownSection(id SectionID) bool { return sectionByID[id] }
