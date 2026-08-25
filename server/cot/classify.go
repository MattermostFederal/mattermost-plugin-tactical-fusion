package cot

import "strings"

const atomPrefix = "a-"

const (
	ClassChat    = "chat"
	ClassMedevac = "medevac"
	ClassSensor  = "sensor"
	ClassVideo   = "video"
)

type typeClass struct {
	Code  string
	Class string
}

// typeClasses is pass one, and a match here is final.
//
// Ordered and matched case-sensitively, because case is part of a CoT code
// everywhere else in this package and a classify that folded it would disagree
// with the label rendered beside it. A row matches the bare code or anything
// below it, which is why b-t-f catches b-t-f-r and b-t-fx catches nothing.
var typeClasses = []typeClass{
	{Code: "b-t-f", Class: ClassChat},
	{Code: "b-r-f-h-c", Class: ClassMedevac},
	{Code: "b-l-p-c", Class: ClassSensor},
}

type blockClass struct {
	Element string
	Class   string
}

// blockClasses is pass two, and it may only promote an event pass one left
// unclassified.
//
// A single ordered table of "type matches or block present" was the obvious
// design and is wrong: under it a hostile contact carrying an empty <__chat/>
// classifies as chat, which is ten author-chosen bytes re-shaping somebody
// else's contact report.
var blockClasses = []blockClass{
	{Element: "_medevac_", Class: ClassMedevac},
	{Element: "__chat", Class: ClassChat},
	{Element: "sensor", Class: ClassSensor},
	{Element: "__video", Class: ClassVideo},
}

func classify(event Event) string {
	for _, row := range typeClasses {
		if event.Type == row.Code || strings.HasPrefix(event.Type, row.Code+"-") {
			return row.Class
		}
	}

	// An atom is a track, with an affiliation the card colours it by, and
	// element presence may never re-shape one. That is the whole spoof: ten
	// author-chosen bytes turning somebody's hostile contact report into a
	// message from a named sender.
	if strings.HasPrefix(event.Type, atomPrefix) {
		return ""
	}

	for _, row := range blockClasses {
		if hasBlock(event, row.Element) {
			return row.Class
		}
	}

	return ""
}

func hasBlock(event Event, element string) bool {
	for _, block := range event.Detail.Blocks {
		if block.Name == element {
			return true
		}
	}
	return false
}

// Classes is every class this build writes, sorted, for the sync test.
func Classes() []string {
	return []string{ClassChat, ClassMedevac, ClassSensor, ClassVideo}
}
