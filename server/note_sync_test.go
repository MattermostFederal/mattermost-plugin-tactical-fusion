package main

import (
	"strconv"
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/note"
)

func TestWebappNoteShapeMatches(t *testing.T) {
	source := readWebappFile(t, "decorators", "note", "index.ts")

	for _, want := range []string{
		"type: '" + note.Type + "'",
		"params.get('" + note.ParamValue + "')",
		"export const MAX_NOTE_RUNES = " + strconv.Itoa(note.MaxNoteRunes) + ";",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the webapp's note/index.ts has no %q", want)
		}
	}
}
