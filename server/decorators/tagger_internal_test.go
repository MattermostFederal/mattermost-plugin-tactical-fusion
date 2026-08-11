package decorators

import "testing"

// Merging matters independently of the bail-out gate: dropping a range because
// it overlapped an earlier one meant a construct could lose protection entirely
// and have a link written into its interior.
func TestMergeRanges(t *testing.T) {
	cases := []struct {
		name  string
		input []byteRange
		want  []byteRange
	}{
		{"empty", nil, nil},
		{"single", []byteRange{{0, 5}}, []byteRange{{0, 5}}},
		{"disjoint stay separate", []byteRange{{0, 5}, {10, 15}}, []byteRange{{0, 5}, {10, 15}}},
		{"overlapping merge", []byteRange{{0, 10}, {5, 15}}, []byteRange{{0, 15}}},
		{"nested collapses", []byteRange{{0, 20}, {5, 10}}, []byteRange{{0, 20}}},
		{"adjacent merge", []byteRange{{0, 5}, {5, 10}}, []byteRange{{0, 10}}},
		{"unsorted input", []byteRange{{10, 20}, {0, 5}, {15, 30}, {5, 8}}, []byteRange{{0, 8}, {10, 30}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mergeRanges(tc.input)

			if len(got) != len(tc.want) {
				t.Fatalf("mergeRanges() = %v, want %v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("mergeRanges() = %v, want %v", got, tc.want)
				}
			}
		})
	}
}
