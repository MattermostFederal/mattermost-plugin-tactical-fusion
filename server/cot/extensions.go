package cot

import (
	"slices"
	"strings"
)

const (
	detailElement   = "detail"
	remarksElement  = "remarks"
	linkElement     = "link"
	flowTagsElement = "_flow-tags_"
	flowVersionAttr = "version"
	xmlnsAttr       = "xmlns"
)

const (
	unitMeters  = " m"
	unitDegrees = "°"
	unitPercent = "%"
	unitColor   = "#"
	unitDbm     = " dBm"

	// unitHashCount reads a JSON array of content hashes and yields how many
	// there are. The hashes themselves are longer than a field, so storing them
	// raw truncates mid-hash into a value that looks like a hash and is not.
	unitHashCount = "[]"
)

const presenceValue = "stated"

// maxCotFlowTags caps the processing path, and drops from the front. Flow tags
// are appended, so document order is oldest first.
const maxCotFlowTags = 16

type Attr struct {
	Name string
	Key  string
	Unit string
}

type Extension struct {
	Element string
	Parent  string
	Prefix  string
	Attrs   []Attr
}

var extensions = []Extension{
	{
		Element: "contact",
		Parent:  detailElement,
		Prefix:  "contact",
		Attrs:   []Attr{{Name: "endpoint", Key: "endpoint"}},
	},
	{
		Element: "uid",
		Parent:  detailElement,
		Prefix:  "uid_extra",
		Attrs:   []Attr{{Name: "Droid", Key: "droid"}},
	},
	{
		Element: "takv",
		Parent:  detailElement,
		Prefix:  "takv",
		Attrs: []Attr{
			{Name: "device", Key: "device"},
			{Name: "platform", Key: "platform"},
			{Name: "os", Key: "os"},
			{Name: "version", Key: "version"},
		},
	},
	{
		Element: "precisionlocation",
		Parent:  detailElement,
		Prefix:  "precision",
		Attrs: []Attr{
			{Name: "geopointsrc", Key: "geopointsrc"},
			{Name: "altsrc", Key: "altsrc"},
			{Name: "pdop", Key: "pdop"},
			{Name: "hdop", Key: "hdop"},
			{Name: "vdop", Key: "vdop"},
		},
	},
	{
		Element: "archive",
		Parent:  detailElement,
		Prefix:  "archive",
	},
	{
		Element: "usericon",
		Parent:  detailElement,
		Prefix:  "usericon",
		Attrs:   []Attr{{Name: "iconsetpath", Key: "iconsetpath"}},
	},
	{
		Element: "color",
		Parent:  detailElement,
		Prefix:  "color",
		Attrs:   []Attr{{Name: "argb", Key: "argb", Unit: unitColor}},
	},
	{
		Element: "track",
		Parent:  detailElement,
		Prefix:  "track",
		Attrs:   []Attr{{Name: "slope", Key: "slope", Unit: unitDegrees}},
	},
	{
		Element: "status",
		Parent:  detailElement,
		Prefix:  "status",
		Attrs: []Attr{
			{Name: "battery", Key: "battery", Unit: unitPercent},
			{Name: "readiness", Key: "readiness"},
		},
	},
	{
		Element: "Attitude",
		Parent:  detailElement,
		Prefix:  "attitude",
		Attrs: []Attr{
			{Name: "roll", Key: "roll", Unit: unitDegrees},
			{Name: "pitch", Key: "pitch", Unit: unitDegrees},
			{Name: "yaw", Key: "yaw", Unit: unitDegrees},
		},
	},
	{
		Element: "sensor",
		Parent:  detailElement,
		Prefix:  "sensor",
		Attrs: []Attr{
			{Name: "azimuth", Key: "azimuth", Unit: unitDegrees},
			{Name: "elevation", Key: "elevation", Unit: unitDegrees},
			{Name: "range", Key: "range", Unit: unitMeters},
			{Name: "fov", Key: "fov", Unit: unitDegrees},
			{Name: "vfov", Key: "vfov", Unit: unitDegrees},
			{Name: "roll", Key: "roll", Unit: unitDegrees},
			{Name: "model", Key: "model"},
		},
	},
	{
		Element: "__video",
		Parent:  detailElement,
		Prefix:  "video",
		Attrs: []Attr{
			{Name: "uid", Key: "uid"},
			{Name: "url", Key: "url"},
		},
	},
	{
		Element: "ConnectionEntry",
		Parent:  "__video",
		Prefix:  "video_conn",
		Attrs: []Attr{
			{Name: "address", Key: "address"},
			{Name: "port", Key: "port"},
			{Name: "protocol", Key: "protocol"},
			{Name: "path", Key: "path"},
		},
	},
	{
		Element: "__chat",
		Parent:  detailElement,
		Prefix:  "chat",
		Attrs: []Attr{
			{Name: "senderCallsign", Key: "sender"},
			{Name: "chatroom", Key: "room"},
			{Name: "id", Key: "id"},
			{Name: "parent", Key: "parent"},
			{Name: "groupOwner", Key: "group_owner"},
		},
	},
	{
		Element: "chatgrp",
		Parent:  "__chat",
		Prefix:  "chatgrp",
		Attrs: []Attr{
			{Name: "uid0", Key: "uid0"},
			{Name: "uid1", Key: "uid1"},
			{Name: "id", Key: "id"},
		},
	},
	{
		Element: "link_attr",
		Parent:  detailElement,
		Prefix:  "route",
		Attrs: []Attr{
			{Name: "routetype", Key: "type"},
			{Name: "planningmethod", Key: "planning"},
			{Name: "method", Key: "method"},
			{Name: "direction", Key: "direction"},
			{Name: "order", Key: "order"},
		},
	},
	{
		Element: "__chatReceipt",
		Parent:  detailElement,
		Prefix:  "chat_receipt",
		Attrs: []Attr{
			{Name: "id", Key: "id"},
			{Name: "chatroom", Key: "room"},
			{Name: "ackuid", Key: "ack"},
			{Name: "senderCallsign", Key: "sender"},
		},
	},
	{
		Element: "__serverdestination",
		Parent:  detailElement,
		Prefix:  "destination",
		Attrs:   []Attr{{Name: "destinations", Key: "servers"}},
	},
	{
		Element: "_radio",
		Parent:  detailElement,
		Prefix:  "radio",
		Attrs: []Attr{
			{Name: "rssi", Key: "rssi", Unit: unitDbm},
			{Name: "gps", Key: "gps"},
		},
	},
	{
		Element: "__geofence",
		Parent:  detailElement,
		Prefix:  "geofence",
		Attrs: []Attr{
			{Name: "monitor", Key: "monitor"},
			{Name: "trigger", Key: "trigger"},
			{Name: "tracking", Key: "tracking"},
			{Name: "elevationMonitored", Key: "elevation"},
			{Name: "minElevation", Key: "min", Unit: unitMeters},
			{Name: "maxElevation", Key: "max", Unit: unitMeters},
			{Name: "boundingSphere", Key: "sphere", Unit: unitMeters},
		},
	},
	{
		Element: "attachment_list",
		Parent:  detailElement,
		Prefix:  "attachments",
		Attrs:   []Attr{{Name: "hashes", Key: "count", Unit: unitHashCount}},
	},
	{
		Element: "TakControl",
		Parent:  detailElement,
		Prefix:  "takcontrol",
	},
	{
		Element: "TakProtocolSupport",
		Parent:  "TakControl",
		Prefix:  "takcontrol_support",
		Attrs:   []Attr{{Name: "version", Key: "version"}},
	},
	{
		Element: "TakRequest",
		Parent:  "TakControl",
		Prefix:  "takcontrol_request",
		Attrs:   []Attr{{Name: "version", Key: "version"}},
	},
	{
		Element: "TakResponse",
		Parent:  "TakControl",
		Prefix:  "takcontrol_response",
		Attrs:   []Attr{{Name: "status", Key: "status"}},
	},
	{
		Element: "_medevac_",
		Parent:  detailElement,
		Prefix:  "medevac",
		Attrs: []Attr{
			{Name: "title", Key: "title"},
			{Name: "urgent", Key: "urgent"},
			{Name: "priority", Key: "priority"},
			{Name: "routine", Key: "routine"},
			{Name: "litter", Key: "litter"},
			{Name: "ambulatory", Key: "ambulatory"},
			{Name: "casevac", Key: "casevac"},
			{Name: "freq", Key: "freq"},
			{Name: "security", Key: "security"},
			{Name: "Security", Key: "security"},
			{Name: "hlz_marking", Key: "hlz_marking"},
			{Name: "terrain_none", Key: "terrain_none"},
			{Name: "equipment_detail", Key: "equipment_detail"},
			{Name: "equipment_none", Key: "equipment_none"},
			{Name: "zone_prot_selection", Key: "zone_prot_selection"},
			{Name: "nationality", Key: "nationality"},
			{Name: "nbc", Key: "nbc"},
			{Name: "medline_remarks", Key: "medline_remarks"},
		},
	},
}

// Extensions is the registry, for the tests that build a fixture from it rather
// than listing what they think it holds.
func Extensions() []Extension {
	return slices.Clone(extensions)
}

func extensionFor(element, parent string) (Extension, bool) {
	for _, ext := range extensions {
		if ext.Element == element && ext.Parent == parent {
			return ext, true
		}
	}
	return Extension{}, false
}

// extensionByElement finds the entry a parsed Block belongs to.
//
// Element names are unique across the registry, which TestRegistryElementsAreUnique
// holds, so a Block need not carry its parent for the props builder to find it.
func extensionByElement(element string) (Extension, bool) {
	for _, ext := range extensions {
		if ext.Element == element {
			return ext, true
		}
	}
	return Extension{}, false
}

// PropKeys is every key the registry can write, sorted.
//
// Exported so TestEventPropsKeysAreClosed can assert an event's top-level key
// set against it rather than against a list somebody maintains by hand.
func PropKeys() []string {
	var keys []string

	for _, ext := range extensions {
		if len(ext.Attrs) == 0 {
			keys = append(keys, ext.Prefix)
			continue
		}
		for _, attr := range ext.Attrs {
			key := ext.Prefix + "_" + attr.Key
			if !slices.Contains(keys, key) {
				keys = append(keys, key)
			}
		}
	}

	slices.Sort(keys)
	return keys
}

// FixtureDetail is a <detail> block exercising everything this build reads.
//
// Exported for the tests that must not narrow as the registry grows: the
// cross-language guard builds its event from this rather than from a
// hand-written fixture somebody has to remember to extend. Nothing drops a key
// from it, because registry names are closed and a repeat is first-wins.
func FixtureDetail() string {
	var out strings.Builder

	for _, ext := range extensions {
		if ext.Parent != detailElement {
			continue
		}

		out.WriteString(openTag(ext))
		for _, child := range extensions {
			if child.Parent == ext.Element {
				out.WriteString(openTag(child) + "</" + child.Element + ">")
			}
		}
		out.WriteString("</" + ext.Element + ">")
	}

	out.WriteString(`<__group name="Cyan" role="Team Member"/>`)
	out.WriteString(`<link uid="ANDROID-88" type="a-f-G" relation="p-p" parent_callsign="ALPHA"/>`)
	out.WriteString(`<_flow-tags_ version="0.2" TAK-Server-Prod="2026-08-23T20:10:00Z"/>`)
	out.WriteString(`<remarks>holding at checkpoint</remarks>`)

	return out.String()
}

// fixtureExtras are the attributes the typed Detail fields still read directly,
// on the elements the registry also carries. They are written on the same tag so
// first-wins cannot cost the fixture a key.
var fixtureExtras = map[string]string{
	"contact": ` callsign="DELTA1"`,
	"track":   ` speed="3.2" course="180.0"`,
}

func openTag(ext Extension) string {
	var out strings.Builder
	out.WriteString("<" + ext.Element)

	for _, attr := range ext.Attrs {
		out.WriteString(" " + attr.Name + `="` + fixtureValue(attr) + `"`)
	}

	out.WriteString(fixtureExtras[ext.Element])
	out.WriteString(">")
	return out.String()
}

func fixtureValue(attr Attr) string {
	switch attr.Unit {
	case unitDegrees:
		return "-12.5"
	case unitMeters:
		return "1500"
	case unitPercent:
		return "87"
	case unitColor:
		return "-65536"
	case unitDbm:
		return "-71"
	case unitHashCount:
		// Escaped, because the fixture writes this inside a double quoted XML
		// attribute and JSON needs its own quotes back after decoding.
		return "[&quot;a&quot;,&quot;b&quot;]"
	default:
		return "v-" + attr.Key
	}
}
