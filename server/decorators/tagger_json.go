package decorators

import "strings"

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
