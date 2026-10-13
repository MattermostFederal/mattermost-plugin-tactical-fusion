package cyber

import "testing"

func TestEveryKindIsKnownAndLabeled(t *testing.T) {
	seen := map[string]Kind{}
	for _, kind := range Kinds {
		if !kind.Known() {
			t.Errorf("%q is listed but not known", kind)
		}
		label := kind.Label()
		if label == "" {
			t.Errorf("%q has no label", kind)
		}
		if other, dup := seen[label]; dup {
			t.Errorf("%q and %q share the label %q", other, kind, label)
		}
		seen[label] = kind
	}
}

func TestAnUnknownKindHasNoLabel(t *testing.T) {
	for _, kind := range []Kind{"", "CVE", "domain"} {
		if kind.Known() {
			t.Errorf("%q reads as known", kind)
		}
		if got := kind.Label(); got != "" {
			t.Errorf("%q label = %q, want empty", kind, got)
		}
	}
}
