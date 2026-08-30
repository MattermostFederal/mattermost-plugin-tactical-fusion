package decorators

import "testing"

func TestSoleObjectSpanFindsTheWholeObject(t *testing.T) {
	block, ok := SoleObjectSpan(`before {"a":{"b":1}} after`)
	if !ok {
		t.Fatal("no object found")
	}
	if block.Body != `{"a":{"b":1}}` {
		t.Errorf("Body = %q", block.Body)
	}
	if block.Lead != "before " || block.Trail != " after" {
		t.Errorf("Lead = %q Trail = %q", block.Lead, block.Trail)
	}
}

func TestSoleObjectSpanIgnoresBracesInStrings(t *testing.T) {
	block, ok := SoleObjectSpan(`{"name":"a } brace","b":2}`)
	if !ok {
		t.Fatal("no object found")
	}
	if block.Body != `{"name":"a } brace","b":2}` {
		t.Errorf("Body = %q", block.Body)
	}
}

func TestSoleObjectSpanHonorsEscapedQuotes(t *testing.T) {
	block, ok := SoleObjectSpan(`{"name":"say \"}\" now","b":2}`)
	if !ok {
		t.Fatal("no object found")
	}
	if block.Body != `{"name":"say \"}\" now","b":2}` {
		t.Errorf("Body = %q", block.Body)
	}
}

func TestSoleObjectSpanRefusesAnUnclosedObject(t *testing.T) {
	if _, ok := SoleObjectSpan(`{"a":1`); ok {
		t.Fatal("an unclosed object was accepted")
	}
}

func TestSoleObjectSpanRefusesAMessageWithNoObject(t *testing.T) {
	if _, ok := SoleObjectSpan("nothing to see here"); ok {
		t.Fatal("a message with no object was accepted")
	}
}

func TestSoleObjectSpanNeverReachesIntoCode(t *testing.T) {
	cases := map[string]string{
		"a fenced block":    "```\n{\"a\":1}\n```",
		"an inline span":    "look at `{\"a\":1}` here",
		"an indented block": "    {\"a\":1}",
	}

	for name, message := range cases {
		t.Run(name, func(t *testing.T) {
			if _, ok := SoleObjectSpan(message); ok {
				t.Fatalf("read an object out of code: %q", message)
			}
		})
	}
}
