package cyber

import (
	"testing"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
)

func newTagger(t *testing.T) *decorators.Tagger {
	t.Helper()

	registry, err := decorators.NewDefaultRegistry(&Decorator{})
	if err != nil {
		t.Fatalf("registry: %v", err)
	}

	return &decorators.Tagger{Registry: registry, URLPrefix: "/plugins/tf/decorate"}
}

func decorate(t *testing.T, message string) string {
	t.Helper()
	return newTagger(t).Decorate(message, time.Now().UTC())
}

func decorated(t *testing.T, message string) bool {
	t.Helper()
	return decorate(t, message) != message
}
