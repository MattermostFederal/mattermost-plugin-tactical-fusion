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

const (
	eventElement = "event"
	pointElement = "point"
	groupElement = "__group"

	pointDepth       = 2
	detailChildDepth = 3
	nestedChildDepth = 4
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
	ErrNullPrefix = errors.New("cot: source binds a namespace prefix to nothing")
)

// budget bounds the decode and is also the parent stack.
//
// The two are one structure because they have to move together: readText
// swallows the remarks subtree and calls enter itself, so a stack kept
// separately in Parse's loop desyncs the first time remarks carry markup, and
// the symptom is a later sibling attributed to the wrong parent.
type budget struct {
	path     []string
	accepted []bool
	elements int
}

func (b *budget) enter(start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space == xmlnsAttr && attr.Value == "" {
			return ErrNullPrefix
		}
	}

	b.elements += 1 + len(start.Attr)
	if b.elements > maxCotElements {
		return ErrTooMany
	}

	if len(b.path)+1 > maxCotDepth {
		return ErrTooDeep
	}

	b.path = append(b.path, qualifiedName(start.Name))
	b.accepted = append(b.accepted, false)
	return nil
}

// qualifiedName keeps the namespace on the stack.
//
// Pushing Name.Local alone enforced the namespace rule on the element being
// matched and not on its ancestors, so <x:detail> was indistinguishable from
// <detail> to every parent test below it and everything inside it was read as
// the event's own. The name is not compared to anything but the bare element
// names the registry declares, so a namespaced ancestor can never match one.
func qualifiedName(name xml.Name) string {
	if name.Space == "" {
		return name.Local
	}
	return name.Space + ":" + name.Local
}

func (b *budget) leave() {
	if len(b.path) > 0 {
		b.path = b.path[:len(b.path)-1]
		b.accepted = b.accepted[:len(b.accepted)-1]
	}
}

// accept marks the element currently open as one this build read as a block.
func (b *budget) accept() {
	if len(b.accepted) > 0 {
		b.accepted[len(b.accepted)-1] = true
	}
}

// parentAccepted reports whether the element enclosing the one currently open
// was itself accepted.
//
// This has to be about the instance, not the name. A flat seen-set answered
// "some element with this name was accepted somewhere earlier in this event",
// which let a legitimate <detail><__video/> earlier in the document vouch for a
// second <__video> parked outside <detail>, and let a repeated <__chat> that
// first-wins had rejected still contribute its <chatgrp>.
func (b *budget) parentAccepted() bool {
	if len(b.accepted) < 2 {
		return false
	}
	return b.accepted[len(b.accepted)-2]
}

func (b *budget) depth() int { return len(b.path) }

func (b *budget) parent() string {
	if len(b.path) < 2 {
		return ""
	}
	return b.path[len(b.path)-2]
}

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

// Block is one registered <detail> extension, as it was written.
type Block struct {
	Name  string
	Attrs map[string]string
}

// FlowTag is one hop in the processing path, in document order.
type FlowTag struct {
	System string
	Time   string
}

type Detail struct {
	Callsign string
	Group    string
	Role     string
	Speed    string
	Course   string
	Remarks  string
	Links    []Link
	Blocks   []Block
	Flow     []FlowTag
	Shape    Geometry
	Route    Geometry
	Unknown  int
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
	seen := map[string]bool{}
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
			if counts.depth() == 0 && strings.TrimSpace(string(t)) != "" {
				return nil, ErrTrailing
			}

		case xml.StartElement:
			if err := counts.enter(t); err != nil {
				return nil, err
			}

			if counts.depth() == 1 {
				if len(events) == maxCotEvents {
					return nil, ErrManyEvents
				}
				if t.Name.Local != eventElement || t.Name.Space != "" {
					return nil, ErrNotEvent
				}

				events = append(events, Event{})
				readEvent(&events[len(events)-1], t)
				seen = map[string]bool{}
				continue
			}

			// Into the event being read. Depth only leaves 1 by closing a root,
			// so a child at depth 2 always has one open above it.
			consumed, err := readChild(&events[len(events)-1], t, decoder, &counts, seen)
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
		if attr.Name.Space != "" {
			continue
		}

		switch attr.Name.Local {
		case "uid":
			setOnce(&event.UID, attr.Value)
		case "type":
			setOnce(&event.Type, attr.Value)
		case "how":
			setOnce(&event.How, attr.Value)
		case "time":
			setOnce(&event.Time, attr.Value)
		case "start":
			setOnce(&event.Start, attr.Value)
		case "stale":
			setOnce(&event.Stale, attr.Value)
		}
	}
}

// readChild reads one element inside an event.
//
// Everything below <detail> is matched on its parent as well as its name, and
// only in no namespace. Both were open before: a <contact> nested anywhere
// became the callsign, a <link> inside <__video> became a relation, and
// <x:contact> was read as though it were the event's own.
func readChild(event *Event, start xml.StartElement, decoder *xml.Decoder, counts *budget, seen map[string]bool) (bool, error) {
	depth := counts.depth()
	local := start.Name.Local

	if start.Name.Space != "" {
		return false, nil
	}

	if depth == pointDepth {
		if local == pointElement {
			readPoint(&event.Point, start)
		}
		return false, nil
	}

	if depth == detailChildDepth && counts.parent() == detailElement {
		return readDetailChild(event, start, decoder, counts, seen)
	}

	// Accepted on what this element filled, not on the geometry being non-empty.
	// Keyed on the latter, a <polyline> after an <ellipse> was marked accepted
	// and its vertices were read into the ellipse.
	if depth == shapeChildDepth && counts.parent() == shapeElement && counts.parentAccepted() {
		if readShapeChild(&event.Detail.Shape, start) {
			counts.accept()
		}
		return false, nil
	}

	if depth == vertexDepth && local == vertexElement && counts.parent() == polylineElement && counts.parentAccepted() {
		event.Detail.Shape.addVertex(attrValue(start, "lat"), attrValue(start, "lon"))
		return false, nil
	}

	if depth == nestedChildDepth && counts.parentAccepted() {
		if ext, ok := extensionFor(local, counts.parent()); ok && ext.Parent != detailElement {
			addBlock(event, ext, start, counts, seen)
		}
		return false, nil
	}

	return false, nil
}

func readDetailChild(event *Event, start xml.StartElement, decoder *xml.Decoder, counts *budget, seen map[string]bool) (bool, error) {
	local := start.Name.Local

	switch local {
	case remarksElement:
		text, err := readText(decoder, counts)
		if err != nil {
			return false, err
		}
		if !seen[local] {
			seen[local] = true
			event.Detail.Remarks = text
		}
		return true, nil

	case linkElement:
		// A route's points are links too, and are told apart by carrying a
		// point rather than by the event's type. Separate caps, so a long route
		// cannot cost the reader the "Sent by" row.
		if raw := attrValue(start, routePointAttr); raw != "" {
			if vertex, ok := routeVertex(raw); ok {
				addRouteVertex(&event.Detail.Route, vertex)
				return false, nil
			}
		}

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
		return false, nil

	case groupElement:
		if !seen[local] {
			seen[local] = true
			event.Detail.Group = attrValue(start, "name")
			event.Detail.Role = attrValue(start, "role")
		}
		return false, nil

	case shapeElement:
		if !seen[local] {
			seen[local] = true
			counts.accept()
		}
		return false, nil

	case flowTagsElement:
		if !seen[local] {
			seen[local] = true
			event.Detail.Flow = readFlowTags(start)
		}
		return false, nil

	case "contact":
		if !seen[local] {
			event.Detail.Callsign = attrValue(start, "callsign")
		}

	case "track":
		if !seen[local] {
			event.Detail.Speed = attrValue(start, "speed")
			event.Detail.Course = attrValue(start, "course")
		}
	}

	// Everything this build reads has already returned above or is a registry
	// entry, so anything reaching here is an element it does not know.
	ext, ok := extensionFor(local, detailElement)
	if !ok {
		event.Detail.Unknown++
		return false, nil
	}

	addBlock(event, ext, start, counts, seen)
	return false, nil
}

// addBlock records one registered element, first occurrence wins.
//
// First-wins matches attrValue, which was already first-wins while a repeated
// element was last-wins because each arm assigned. One rule now. It is also
// what keeps a registry-derived sync fixture honest, since a repeat can never
// add a key.
func addBlock(event *Event, ext Extension, start xml.StartElement, counts *budget, seen map[string]bool) {
	if seen[ext.Element] {
		return
	}
	seen[ext.Element] = true
	counts.accept()

	attrs := make(map[string]string, len(ext.Attrs))
	for _, attr := range ext.Attrs {
		if value := attrValue(start, attr.Name); value != "" {
			if _, held := attrs[attr.Key]; !held {
				attrs[attr.Key] = value
			}
		}
	}

	event.Detail.Blocks = append(event.Detail.Blocks, Block{Name: ext.Element, Attrs: attrs})
}

// readFlowTags reads the processing path, whose attribute NAMES are the data.
//
// Namespace declarations arrive as attributes and Name.Local on xmlns:x is "x",
// so an unfiltered reader renders a namespace URI as a hop's timestamp. version
// is excluded by name or the table gains a system called "version".
func readFlowTags(start xml.StartElement) []FlowTag {
	var tags []FlowTag

	for _, attr := range start.Attr {
		if attr.Name.Space == xmlnsAttr || attr.Name.Local == xmlnsAttr {
			continue
		}
		if attr.Name.Space != "" || strings.EqualFold(attr.Name.Local, flowVersionAttr) {
			continue
		}

		tags = append(tags, FlowTag{System: attr.Name.Local, Time: attr.Value})
	}

	// Dropped from the front. Flow tags are appended, so document order is
	// oldest first and the tail is what a reader is looking for.
	if len(tags) > maxCotFlowTags {
		tags = tags[len(tags)-maxCotFlowTags:]
	}

	return tags
}

func readPoint(point *Point, start xml.StartElement) {
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}

		switch attr.Name.Local {
		case "lat":
			setOnce(&point.Lat, attr.Value)
		case "lon":
			setOnce(&point.Lon, attr.Value)
		case "hae":
			setOnce(&point.HAE, attr.Value)
		case "ce":
			setOnce(&point.CE, attr.Value)
		case "le":
			setOnce(&point.LE, attr.Value)
		}
	}
}

func setOnce(field *string, value string) {
	if *field == "" {
		*field = value
	}
}

func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == name {
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
			if err := counts.enter(t); err != nil {
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
