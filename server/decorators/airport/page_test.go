package airport

import (
	"strings"
	"testing"
)

func TestRenderBodyFallsBackToTheIdentWhenTheNameIsEmpty(t *testing.T) {
	body := renderBody("KIND", Details{Ident: "KIND", Place: "Indianapolis, IN, US"}, true)

	if !strings.Contains(body, `<p class="name">KIND</p>`) {
		t.Errorf("body carries no name heading:\n%s", body)
	}
}

func TestRenderBodyEscapesAnIdentThisBuildDoesNotHold(t *testing.T) {
	body := renderBody(`<script>x</script>`, Details{}, false)

	if strings.Contains(body, "<script>") {
		t.Errorf("the ident reached the page as markup:\n%s", body)
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Errorf("the ident was not escaped:\n%s", body)
	}
}
