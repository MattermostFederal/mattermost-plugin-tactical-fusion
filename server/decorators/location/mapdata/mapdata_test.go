package mapdata

import "testing"

func TestDataDecodes(t *testing.T) {
	t.Logf("countries=%d", len(Countries))

	if len(Countries) == 0 {
		t.Fatal("the generated basemap decoded to nothing")
	}

	for _, c := range Countries {
		if c.Name == "" {
			t.Fatal("a country decoded with no name")
		}
		if len(c.Polys) == 0 {
			t.Fatalf("%s decoded with no polygons", c.Name)
		}
	}
}
