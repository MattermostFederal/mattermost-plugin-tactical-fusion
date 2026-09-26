package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
)

func TestDecoratePostSkipsAMessageNoServerCanStore(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	token := "1.1.1.1 "
	message := strings.Repeat(token, maxStorableRunes/len(token)+1)

	if got := p.decoratePost(&model.Post{Message: message}, hookRef); got != nil {
		t.Fatalf("a %d rune message was decorated; the server refuses it after the hook anyway", len(message))
	}
}

func TestDecoratePostStillStripsAForgedTypeFromAnUnstorableMessage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	post := &model.Post{Message: strings.Repeat("x", maxStorableRunes+1), Type: cot.PostType, UserId: testUserID}
	post.AddProp(cot.PropsKey, map[string]any{"version": 1})

	got := p.decoratePost(post, hookRef)
	if got == nil {
		t.Fatal("the forged type was left on the post")
	}
	if got.Type != "" {
		t.Errorf("Type = %q, want the forged type stripped", got.Type)
	}
	if _, ok := got.GetProps()[cot.PropsKey]; ok {
		t.Error("the forged blob survived")
	}
}
