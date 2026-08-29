package decorators

import "strings"

/*
 * SoleObjectSpan finds a JSON object written into a message with no fence.
 *
 * The sibling of SoleElementSpan, and it exists for the same reason: an author
 * who pastes a document without wrapping it should still get the card. It is a
 * BRACE MATCHER and nothing else. Deciding whether the span is really the
 * format a caller wants is the caller's parse, exactly as it is for the element
 * scanner, so this package does not grow a JSON reader.
 *
 * Spans that the author marked as code are refused, for the reason
 * SoleElementSpan refuses them: an author who fenced something has said it is
 * code, and reading it anyway is the corruption protected ranges exist to stop.
 *
 * Braces inside STRINGS are skipped, with escapes honored. Without that a
 * document carrying "}" in a property value ends the span early and the caller
 * is handed a truncated object, which parses as a failure rather than as the
 * document the author wrote.
 */
func SoleObjectSpan(message string) (FencedBlock, bool) {
	start := strings.IndexByte(message, '{')
	if start < 0 {
		return FencedBlock{}, false
	}

	end, ok := objectEnd(message, start)
	if !ok {
		return FencedBlock{}, false
	}

	if overlapsAny(byteRange{start, end}, codeRanges(message)) {
		return FencedBlock{}, false
	}

	return FencedBlock{
		Body:  message[start:end],
		Lead:  message[:start],
		Trail: message[end:],
	}, true
}

// objectEnd returns the offset just past the brace that closes the one opened
// at start, or false when the message never closes it.
func objectEnd(message string, start int) (int, bool) {
	depth := 0
	inString := false
	escaped := false

	for i := start; i < len(message); i++ {
		c := message[i]

		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}

		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}

	return 0, false
}
