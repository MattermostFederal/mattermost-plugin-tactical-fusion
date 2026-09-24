package cyber

import (
	"reflect"
	"strings"
	"testing"
)

func TestDescribeVectorDecodesACVSS31Vector(t *testing.T) {
	got := DescribeVector("CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H")

	want := []VectorMetric{
		{"Attack vector", "Network", true},
		{"Attack complexity", "Low", true},
		{"Privileges required", "None", true},
		{"User interaction", "None", true},
		{"Scope", "Changed", true},
		{"Confidentiality", "High", true},
		{"Integrity", "High", true},
		{"Availability", "High", true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestDescribeVectorMarksOnlyTheMostDangerousValueSevere(t *testing.T) {
	got := DescribeVector("CVSS:3.0/AV:P/AC:H/PR:H/UI:R/S:U/C:L/I:N/A:N")

	for _, metric := range got {
		if metric.Severe {
			t.Errorf("%s: %s was marked severe", metric.Metric, metric.Value)
		}
	}
	if len(got) != 8 || got[0].Value != "Physical" || got[3].Value != "Required" {
		t.Fatalf("got %+v", got)
	}
}

func TestDescribeVectorSkipsTheMetricsACVSS4VectorLeavesUndefined(t *testing.T) {
	vector := "CVSS:4.0/AV:N/AC:L/AT:P/PR:N/UI:A/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N/E:X/CR:X/IR:X/AR:X/MAV:X/MAC:X/MAT:X/MPR:X/MUI:X/MVC:X/MVI:X/MVA:X/MSC:X/MSI:X/MSA:X/S:X/AU:X/R:X/V:X/RE:X/U:X"

	got := DescribeVector(vector)

	if len(got) != 11 {
		t.Fatalf("%d metrics, want the 11 base metrics: %+v", len(got), got)
	}
	if got[2] != (VectorMetric{"Attack requirements", "Present", false}) {
		t.Errorf("attack requirements %+v", got[2])
	}
	if got[4] != (VectorMetric{"User interaction", "Active", false}) {
		t.Errorf("user interaction %+v", got[4])
	}
	if got[8] != (VectorMetric{"Confidentiality, subsequent systems", "None", false}) {
		t.Errorf("subsequent confidentiality %+v", got[8])
	}
}

func TestDescribeVectorReadsTheCVSS4ThreatAndModifiedMetrics(t *testing.T) {
	got := DescribeVector("CVSS:4.0/AV:L/AC:L/AT:N/PR:N/UI:N/VC:H/VI:H/VA:H/SC:N/SI:N/SA:N/E:A/MAV:N/AU:Y/U:Red")

	tail := got[len(got)-4:]
	want := []VectorMetric{
		{"Exploit maturity", "Attacked", true},
		{"Modified attack vector", "Network", true},
		{"Automatable", "Yes", true},
		{"Provider urgency", "Red", true},
	}
	if !reflect.DeepEqual(tail, want) {
		t.Fatalf("got %+v\nwant %+v", tail, want)
	}
}

func TestDescribeVectorReadsAnUnprefixedVectorAsCVSS2(t *testing.T) {
	got := DescribeVector("AV:N/AC:L/Au:N/C:C/I:C/A:P")

	want := []VectorMetric{
		{"Attack vector", "Network", true},
		{"Attack complexity", "Low", true},
		{"Authentication", "None", true},
		{"Confidentiality", "Complete", true},
		{"Integrity", "Complete", true},
		{"Availability", "Partial", false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestDescribeVectorShowsWhatItDoesNotKnowAsWritten(t *testing.T) {
	got := DescribeVector("CVSS:3.1/AV:Q/ZZ:9")

	want := []VectorMetric{
		{"Attack vector", "Q", false},
		{"ZZ", "9", false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestDescribeVectorDescribesNothingRatherThanHalfAMalformedVector(t *testing.T) {
	for _, vector := range []string{"", "CVSS:3.1", "CVSS:3.1/AV:N/garbage", "CVSS:9.0/AV:N", "CVSS:3.1/AV:", "AV:N/:L"} {
		if got := DescribeVector(vector); got != nil {
			t.Errorf("DescribeVector(%q) = %+v, want nothing", vector, got)
		}
	}
}

func TestDescribeVectorReadsEveryVectorShapeTheRecordsCarry(t *testing.T) {
	for _, vector := range []string{
		"CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:H/A:H",
		"CVSS:3.0/AV:A/AC:H/PR:L/UI:R/S:U/C:N/I:L/A:N",
		"CVSS:4.0/AV:N/AC:L/AT:N/PR:N/UI:N/VC:H/VI:N/VA:N/SC:N/SI:N/SA:N",
		"AV:L/AC:M/Au:S/C:P/I:N/A:N",
	} {
		for _, metric := range DescribeVector(vector) {
			if metric.Metric == "" || len(metric.Metric) <= 2 && strings.ToUpper(metric.Metric) == metric.Metric {
				t.Errorf("%s: metric %q was left as its code", vector, metric.Metric)
			}
			if len(metric.Value) == 1 {
				t.Errorf("%s: %s value %q was left as its code", vector, metric.Metric, metric.Value)
			}
		}
	}
}
