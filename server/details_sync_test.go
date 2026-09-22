package main

import (
	"strings"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

func TestWebappDetailsLinkLabelMatches(t *testing.T) {
	source := readWebappFile(t, "decorators", "styles.ts")
	if !strings.Contains(source, "export const DETAILS_LINK_LABEL = '"+decorators.DetailsLinkLabel+"';") {
		t.Errorf("the webapp's DETAILS_LINK_LABEL is not %q", decorators.DetailsLinkLabel)
	}
}
