package main

import (
	"reflect"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/cot"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/geojson"
)

func pluginOwnedKeys() []string {
	keys := []string{decorators.PostPropsKey}
	for _, stamped := range stampedTypes {
		keys = append(keys, stamped.propsKey)
	}
	return keys
}

func TestMessageWillBeUpdatedNeverRewritesTheMessage(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	oldPost := &model.Post{Message: "before", Type: cot.PostType}
	oldPost.AddProp(cot.PropsKey, map[string]any{"version": float64(1)})
	newPost := oldPost.Clone()
	newPost.Message = "after 091630Z 34.0561N,118.2500W"
	newPost.AddProp(cot.PropsKey, map[string]any{"version": float64(2)})

	got, reason := p.MessageWillBeUpdated(nil, newPost, oldPost)
	if reason != "" {
		t.Fatalf("an edit was refused: %q", reason)
	}
	if got == nil || got.Message != newPost.Message {
		t.Fatalf("the edited message was not stored verbatim: %+v", got)
	}
}

func TestMessageWillBeUpdatedLeavesAnEditThatTouchesNoPluginFieldAlone(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	oldPost := &model.Post{Message: "before", Type: cot.PostType}
	oldPost.AddProp(cot.PropsKey, map[string]any{"version": float64(1)})
	newPost := oldPost.Clone()
	newPost.Message = "after"
	newPost.AddProp("disable_group_highlight", true)

	if got, _ := p.MessageWillBeUpdated(nil, newPost, oldPost); got != newPost {
		t.Fatalf("an edit that kept every plugin field was not passed through as it arrived: %+v", got)
	}
}

func TestAPropsOnlyEditCannotReplaceAStampedBlob(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, key := range pluginOwnedKeys() {
		t.Run(key, func(t *testing.T) {
			genuine := map[string]any{"callsign": "GENUINE"}
			oldPost := &model.Post{Message: "event", Type: cot.PostType}
			oldPost.AddProp(key, genuine)
			newPost := oldPost.Clone()
			newPost.AddProp(key, map[string]any{"callsign": "FORGED"})

			got, _ := p.MessageWillBeUpdated(nil, newPost, oldPost)
			if got == nil {
				t.Fatal("the forged blob was stored")
			}
			if !reflect.DeepEqual(got.GetProp(key), genuine) {
				t.Fatalf("%s = %v, want the stamped value", key, got.GetProp(key))
			}
			if got.Type != cot.PostType {
				t.Fatalf("Type = %q, want the stamped type kept", got.Type)
			}
		})
	}
}

func TestAnEditCannotRemoveAStampedBlob(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, key := range pluginOwnedKeys() {
		t.Run(key, func(t *testing.T) {
			oldPost := &model.Post{Message: "event"}
			oldPost.AddProp(key, map[string]any{"version": float64(1)})
			newPost := oldPost.Clone()
			newPost.DelProp(key)

			got, _ := p.MessageWillBeUpdated(nil, newPost, oldPost)
			if got == nil || got.GetProp(key) == nil {
				t.Fatalf("%s was removed by an edit", key)
			}
		})
	}
}

func TestAnEditCannotStampAPlainPost(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	for _, stamped := range stampedTypes {
		t.Run(stamped.postType, func(t *testing.T) {
			oldPost := &model.Post{Message: "plain"}
			newPost := oldPost.Clone()
			newPost.Type = stamped.postType
			newPost.AddProp(stamped.propsKey, map[string]any{"version": float64(1)})
			newPost.AddProp(decorators.PostPropsKey, map[string]any{"f": "dd"})

			got, _ := p.MessageWillBeUpdated(nil, newPost, oldPost)
			if got == nil {
				t.Fatal("a forged stamp was stored")
			}
			if got.Type != "" {
				t.Errorf("Type = %q, want the post left unstamped", got.Type)
			}
			for _, key := range []string{stamped.propsKey, decorators.PostPropsKey} {
				if _, ok := got.GetProps()[key]; ok {
					t.Errorf("%s survived the edit", key)
				}
			}
		})
	}
}

func TestAnEditCannotChangeAStampedType(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	oldPost := &model.Post{Message: "event", Type: cot.PostType}
	newPost := oldPost.Clone()
	newPost.Type = geojson.PostType

	got, _ := p.MessageWillBeUpdated(nil, newPost, oldPost)
	if got == nil || got.Type != cot.PostType {
		t.Fatalf("the stamped type was changed by an edit: %+v", got)
	}
}

func TestAnEditLeavesAnotherPluginsTypeAlone(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	oldPost := &model.Post{Message: "x", Type: "custom_other"}
	newPost := oldPost.Clone()
	newPost.Type = "custom_other_v2"

	if got, _ := p.MessageWillBeUpdated(nil, newPost, oldPost); got != newPost {
		t.Fatalf("another plugin's type change was not passed through as it arrived: %+v", got)
	}
}

func TestMessageWillBeUpdatedNeverRejectsAnEdit(t *testing.T) {
	p := newTestPlugin(t, "https://example.com", true)

	stamped := &model.Post{Message: "event", Type: cot.PostType}
	stamped.AddProp(cot.PropsKey, map[string]any{"version": float64(1)})
	forged := stamped.Clone()
	forged.AddProp(cot.PropsKey, map[string]any{"version": float64(2)})
	edited := stamped.Clone()
	edited.Message = "edited"
	plain := &model.Post{Message: "plain"}
	plainEdited := plain.Clone()
	plainEdited.Message = "plain, edited"

	for name, pair := range map[string][2]*model.Post{
		"a plain text edit":         {plainEdited, plain},
		"a stamped post's text":     {edited, stamped},
		"a forged props-only edit":  {forged, stamped},
		"an edit that changes none": {stamped.Clone(), stamped},
	} {
		t.Run(name, func(t *testing.T) {
			got, reason := p.MessageWillBeUpdated(nil, pair[0], pair[1])
			if got == nil || reason != "" {
				t.Fatalf("the edit was rejected: post %v, reason %q", got, reason)
			}
			if got.Message != pair[0].Message {
				t.Fatalf("the message became %q, want %q", got.Message, pair[0].Message)
			}
		})
	}
}
