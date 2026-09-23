package decorators

import (
	"net/http"
	"net/url"
	"regexp"
	"testing"
	"time"
)

type multiDecorator struct {
	postType string
	propsKey string
}

func (d *multiDecorator) Type() string { return "multi" }

func (d *multiDecorator) Patterns() []Pattern {
	return []Pattern{{Regexp: regexp.MustCompile(`\d{4}`)}}
}

func (d *multiDecorator) Parse(value string, _ time.Time) (url.Values, bool) {
	return url.Values{"v": {value}}, true
}

func (d *multiDecorator) RenderPage(w http.ResponseWriter, _ url.Values) {
	w.WriteHeader(http.StatusOK)
}

func (d *multiDecorator) MultiPost() (string, string) { return d.postType, d.propsKey }

func (d *multiDecorator) MultiPostProps(tokens []Token) (map[string]any, bool) {
	return map[string]any{"count": len(tokens)}, true
}

func TestMultiPostTypeRefusesATypeThatCannotBeStored(t *testing.T) {
	for name, d := range map[string]*multiDecorator{
		"no prefix": {postType: "tf_multi", propsKey: "k"},
		"too long":  {postType: "custom_" + "abcdefghijklmnopqrstuvwxyz", propsKey: "k"},
		"no key":    {postType: "custom_tf_multi", propsKey: ""},
		"nothing":   {},
		"declined":  {postType: "", propsKey: "k"},
	} {
		t.Run(name, func(t *testing.T) {
			postType, propsKey := MultiPostType(d)
			if postType != "" || propsKey != "" {
				t.Fatalf("MultiPostType = %q, %q, want both empty", postType, propsKey)
			}
		})
	}

	postType, propsKey := MultiPostType(&multiDecorator{postType: "custom_tf_multi", propsKey: "tactical_fusion_multi"})
	if postType != "custom_tf_multi" || propsKey != "tactical_fusion_multi" {
		t.Fatalf("MultiPostType = %q, %q", postType, propsKey)
	}
}

func TestMultiPostTypeIsEmptyForADecoratorThatDeclaresNone(t *testing.T) {
	postType, propsKey := MultiPostType(&monikerDecorator{})
	if postType != "" || propsKey != "" {
		t.Fatalf("MultiPostType = %q, %q for a decorator that declares none", postType, propsKey)
	}
	if _, ok := MultiPostProps(&monikerDecorator{}, nil); ok {
		t.Fatal("MultiPostProps answered for a decorator that declares none")
	}
}

func TestMultiPostPropsReachTheDecorator(t *testing.T) {
	blob, ok := MultiPostProps(&multiDecorator{}, []Token{{Type: "multi"}, {Type: "multi"}})
	if !ok || blob["count"] != 2 {
		t.Fatalf("MultiPostProps = %v, %v", blob, ok)
	}
}
