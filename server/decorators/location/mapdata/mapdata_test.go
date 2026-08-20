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

func TestDecodePointsRefusesAMalformedRun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		encoded string
	}{
		{"nothing at all", ""},
		{"a pair with no separator", "10 20,30"},
		{"an unreadable longitude delta", "zz+ 20"},
		{"an unreadable latitude delta", "10 zz+"},
		{"an unreadable delta after a good one", "10 20,30 4$"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xs, ys, ok := decodePoints(tc.encoded, adminScale)
			if ok {
				t.Fatalf("decoded %v, %v, want a refusal", xs, ys)
			}
			if xs != nil || ys != nil {
				t.Errorf("returned %v, %v beside the refusal, want nothing", xs, ys)
			}
		})
	}
}

func TestDecodePointsAccumulatesDeltas(t *testing.T) {
	xs, ys, ok := decodePoints("a 1,a 1,-a -1", adminScale)
	if !ok {
		t.Fatal("a well-formed run was refused")
	}
	want := [][2]float64{{10.0 / adminScale, 1.0 / adminScale}, {20.0 / adminScale, 2.0 / adminScale}, {10.0 / adminScale, 1.0 / adminScale}}
	for i, w := range want {
		if xs[i] != w[0] || ys[i] != w[1] {
			t.Errorf("point %d = %v, %v, want %v, %v", i, xs[i], ys[i], w[0], w[1])
		}
	}
}

func TestDecodeCountriesSkipsWhatItCannotRead(t *testing.T) {
	const triangle = "0 0,1m 0,0 1m"

	for _, tc := range []struct {
		name    string
		encoded string
		want    int
	}{
		{"nothing at all", "", 0},
		{"a line with no geometry", "Atlantis", 0},
		{"a line with no name", "|" + triangle, 0},
		{"a ring of two points", "Atlantis|0 0,1m 0", 0},
		{"a ring with an unreadable point", "Atlantis|0 0,1m 0,zz+ 1m", 0},
		{"a sound triangle", "Atlantis|" + triangle, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := decodeCountries(tc.encoded)
			if len(got) != tc.want {
				t.Fatalf("decoded %d countries, want %d", len(got), tc.want)
			}
		})
	}
}

func TestAWholePolygonGoesWhenOneOfItsRingsDoes(t *testing.T) {
	const outer = "0 0,2m 0,0 2m"
	const hole = "1m 1m,-a 0,0 -a"

	sound := decodeCountries("Atlantis|" + outer + ":" + hole)
	if len(sound) != 1 || len(sound[0].Polys) != 1 || len(sound[0].Polys[0]) != 2 {
		t.Fatalf("the sound polygon decoded as %+v, want one polygon of two rings", sound)
	}

	broken := decodeCountries("Atlantis|" + outer + ":1m 1m")
	if len(broken) != 0 {
		t.Errorf("a polygon with a broken hole decoded as %+v, want the whole polygon dropped", broken)
	}
}

func TestBoxContainsIncludesItsEdges(t *testing.T) {
	b := Box{MinX: -10, MinY: -5, MaxX: 10, MaxY: 5}

	for _, tc := range []struct {
		x, y float64
		want bool
	}{
		{0, 0, true},
		{-10, -5, true},
		{10, 5, true},
		{-10.1, 0, false},
		{0, 5.1, false},
		{10.1, 5.1, false},
	} {
		if got := b.Contains(tc.x, tc.y); got != tc.want {
			t.Errorf("Contains(%v, %v) = %v, want %v", tc.x, tc.y, got, tc.want)
		}
	}
}
