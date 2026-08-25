package cot

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// MaxSourceBytes is the largest source this package will read, and is what the
// hook checks an attachment against before asking the filestore for it.
const MaxSourceBytes = 64 * 1024

// MaxEvents is how many events one source may carry, exported so the examples
// command can state the limit rather than repeat the number.
const MaxEvents = 32

// MaxLinks is how many relations one event may declare, exported for the same
// reason as MaxEvents.
const MaxLinks = 16

const (
	maxCotBytes    = MaxSourceBytes
	maxCotDepth    = 32
	maxCotElements = 4096

	// maxCotEvents caps how many events one source may carry.
	//
	// A block of several is ordinary: a batch of position reports, or a set of
	// markers pasted together. A block of hundreds is a feed dump, and every one
	// of them costs props on a post whose whole map must fit inside what the
	// server accepts. Past this the source is refused rather than truncated,
	// because a card showing the first thirty-two of two hundred events is a
	// card that is quietly wrong about what was posted.
	maxCotEvents = MaxEvents

	// maxCotLinks caps the relations one event may declare.
	maxCotLinks = MaxLinks
)

var (
	ErrTooLarge   = errors.New("cot: source exceeds the maximum size")
	ErrNotUTF8    = errors.New("cot: source is not valid UTF-8")
	ErrDirective  = errors.New("cot: source carries an XML directive")
	ErrProcInst   = errors.New("cot: source carries a processing instruction")
	ErrTooDeep    = errors.New("cot: source nests too deeply")
	ErrTooMany    = errors.New("cot: source carries too many elements")
	ErrNotEvent   = errors.New("cot: root element is not an event")
	ErrManyEvents = errors.New("cot: source carries more events than this build reads")
	ErrIncomplete = errors.New("cot: event is missing a uid, a type or a time")
	ErrTrailing   = errors.New("cot: source carries content after the event")
)

type budget struct {
	depth    int
	elements int
}

func (b *budget) enter() error {
	b.elements++
	if b.elements > maxCotElements {
		return ErrTooMany
	}

	b.depth++
	if b.depth > maxCotDepth {
		return ErrTooDeep
	}

	return nil
}

func (b *budget) leave() { b.depth-- }

type Point struct {
	Lat, Lon    string
	HAE, CE, LE string
}

// Link is one relation this event declares to another.
//
// ATAK writes at least one on almost every event: a "p-p" relation naming the
// device that produced it, usually with the sending unit's callsign on it. That
// is the one a reader wants, since it answers "who sent this".
type Link struct {
	UID            string
	Type           string
	Relation       string
	ParentCallsign string
}

type Detail struct {
	Callsign string
	Group    string
	Role     string
	Speed    string
	Course   string
	Remarks  string
	Links    []Link
}

type Event struct {
	UID    string
	Type   string
	How    string
	Time   string
	Start  string
	Stale  string
	Point  Point
	Detail Detail
}

// Parse reads every event the source carries, in the order it wrote them.
//
// A source may hold more than one: a batch of position reports, or a set of
// markers pasted together. Every one has to be complete, because a block whose
// third event is malformed is a block somebody will read as three good ones.
func Parse(source []byte) ([]Event, error) {
	if len(source) > maxCotBytes {
		return nil, ErrTooLarge
	}
	if !utf8.Valid(source) {
		return nil, ErrNotUTF8
	}

	decoder := xml.NewDecoder(bytes.NewReader(source))
	decoder.Strict = true

	var events []Event
	var counts budget
	seenProcInst := false

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}

		switch t := token.(type) {
		case xml.Directive:
			return nil, ErrDirective

		case xml.ProcInst:
			if t.Target != "xml" || seenProcInst || counts.elements > 0 {
				return nil, ErrProcInst
			}
			seenProcInst = true

		case xml.CharData:
			if counts.depth == 0 && strings.TrimSpace(string(t)) != "" {
				return nil, ErrTrailing
			}

		case xml.StartElement:
			if err := counts.enter(); err != nil {
				return nil, err
			}

			if counts.depth == 1 {
				if len(events) == maxCotEvents {
					return nil, ErrManyEvents
				}
				if t.Name.Local != "event" {
					return nil, ErrNotEvent
				}

				events = append(events, Event{})
				readEvent(&events[len(events)-1], t)
				continue
			}

			// Into the event being read. Depth only leaves 1 by closing a root,
			// so a child at depth 2 always has one open above it.
			consumed, err := readChild(&events[len(events)-1], t, decoder, counts.depth, &counts)
			if err != nil {
				return nil, err
			}
			if consumed {
				counts.leave()
			}

		case xml.EndElement:
			counts.leave()
		}
	}

	if len(events) == 0 {
		return nil, ErrNotEvent
	}

	for _, event := range events {
		if event.UID == "" || event.Type == "" || event.Time == "" {
			return nil, ErrIncomplete
		}
	}

	return events, nil
}

func readEvent(event *Event, start xml.StartElement) {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "uid":
			event.UID = attr.Value
		case "type":
			event.Type = attr.Value
		case "how":
			event.How = attr.Value
		case "time":
			event.Time = attr.Value
		case "start":
			event.Start = attr.Value
		case "stale":
			event.Stale = attr.Value
		}
	}
}

func readChild(event *Event, start xml.StartElement, decoder *xml.Decoder, depth int, counts *budget) (bool, error) {
	switch {
	case depth == 2 && start.Name.Local == "point":
		readPoint(&event.Point, start)
	case start.Name.Local == "contact":
		event.Detail.Callsign = attrValue(start, "callsign")
	case start.Name.Local == "__group":
		event.Detail.Group = attrValue(start, "name")
		event.Detail.Role = attrValue(start, "role")
	case start.Name.Local == "track":
		event.Detail.Speed = attrValue(start, "speed")
		event.Detail.Course = attrValue(start, "course")
	case start.Name.Local == "link":
		// Capped, because a link is author-controlled and every one of them
		// costs props on a post that has a budget.
		if len(event.Detail.Links) < maxCotLinks {
			event.Detail.Links = append(event.Detail.Links, Link{
				UID:            attrValue(start, "uid"),
				Type:           attrValue(start, "type"),
				Relation:       attrValue(start, "relation"),
				ParentCallsign: attrValue(start, "parent_callsign"),
			})
		}

	case start.Name.Local == "remarks":
		text, err := readText(decoder, counts)
		if err != nil {
			return false, err
		}
		event.Detail.Remarks = text
		return true, nil
	}

	return false, nil
}

func readPoint(point *Point, start xml.StartElement) {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "lat":
			point.Lat = attr.Value
		case "lon":
			point.Lon = attr.Value
		case "hae":
			point.HAE = attr.Value
		case "ce":
			point.CE = attr.Value
		case "le":
			point.LE = attr.Value
		}
	}
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func readText(decoder *xml.Decoder, counts *budget) (string, error) {
	var text strings.Builder
	nested := 0

	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}

		switch t := token.(type) {
		case xml.CharData:
			text.Write(t)

		case xml.Directive:
			return "", ErrDirective

		case xml.ProcInst:
			return "", ErrProcInst

		case xml.StartElement:
			if err := counts.enter(); err != nil {
				return "", err
			}
			nested++

		case xml.EndElement:
			if nested == 0 {
				return text.String(), nil
			}
			counts.leave()
			nested--
		}
	}
}
