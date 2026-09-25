package cyber

import "strings"

type VectorMetric struct {
	Metric string
	Value  string
	Severe bool
}

type metricSpec struct {
	name   string
	values map[string]string
	severe string
}

const (
	vectorPrefix    = "CVSS:"
	notDefinedValue = "X"
	modifiedPrefix  = "M"
)

var (
	attackVector   = map[string]string{"N": "Network", "A": "Adjacent network", "L": "Local", "P": "Physical"}
	lowHigh        = map[string]string{"L": "Low", "H": "High"}
	noneLowHigh    = map[string]string{"N": "None", "L": "Low", "H": "High"}
	highLowNone    = map[string]string{"H": "High", "L": "Low", "N": "None"}
	requirement    = map[string]string{"H": "High", "M": "Medium", "L": "Low"}
	partialImpact  = map[string]string{"N": "None", "P": "Partial", "C": "Complete"}
	cvss3UI        = map[string]string{"N": "None", "R": "Required"}
	cvss4UI        = map[string]string{"N": "None", "P": "Passive", "A": "Active"}
	cvss3Scope     = map[string]string{"U": "Unchanged", "C": "Changed"}
	cvss4Required  = map[string]string{"N": "None", "P": "Present"}
	cvss2AC        = map[string]string{"L": "Low", "M": "Medium", "H": "High"}
	cvss2Auth      = map[string]string{"N": "None", "S": "Single", "M": "Multiple"}
	cvss4Exploit   = map[string]string{"A": "Attacked", "P": "Proof of concept", "U": "Unreported"}
	cvss4Safety    = map[string]string{"N": "Negligible", "P": "Present"}
	cvss4Automate  = map[string]string{"N": "No", "Y": "Yes"}
	cvss4Recovery  = map[string]string{"A": "Automatic", "U": "User", "I": "Irrecoverable"}
	cvss4Density   = map[string]string{"D": "Diffuse", "C": "Concentrated"}
	cvss4Effort    = map[string]string{"L": "Low", "M": "Moderate", "H": "High"}
	cvss4Urgency   = map[string]string{"Clear": "Clear", "Green": "Green", "Amber": "Amber", "Red": "Red"}
	cvss3Exploit   = map[string]string{"U": "Unproven", "P": "Proof of concept", "F": "Functional", "H": "High"}
	cvss3Remedy    = map[string]string{"O": "Official fix", "T": "Temporary fix", "W": "Workaround", "U": "Unavailable"}
	cvss3Confident = map[string]string{"U": "Unknown", "R": "Reasonable", "C": "Confirmed"}
)

var cvss3Metrics = map[string]metricSpec{
	"AV": {"Attack vector", attackVector, "N"},
	"AC": {"Attack complexity", lowHigh, "L"},
	"PR": {"Privileges required", noneLowHigh, "N"},
	"UI": {"User interaction", cvss3UI, "N"},
	"S":  {"Scope", cvss3Scope, "C"},
	"C":  {"Confidentiality", highLowNone, "H"},
	"I":  {"Integrity", highLowNone, "H"},
	"A":  {"Availability", highLowNone, "H"},
	"E":  {"Exploit maturity", cvss3Exploit, "H"},
	"RL": {"Remediation level", cvss3Remedy, "U"},
	"RC": {"Report confidence", cvss3Confident, ""},
	"CR": {"Confidentiality requirement", requirement, ""},
	"IR": {"Integrity requirement", requirement, ""},
	"AR": {"Availability requirement", requirement, ""},
}

var cvss4Metrics = map[string]metricSpec{
	"AV": {"Attack vector", attackVector, "N"},
	"AC": {"Attack complexity", lowHigh, "L"},
	"AT": {"Attack requirements", cvss4Required, "N"},
	"PR": {"Privileges required", noneLowHigh, "N"},
	"UI": {"User interaction", cvss4UI, "N"},
	"VC": {"Confidentiality, vulnerable system", highLowNone, "H"},
	"VI": {"Integrity, vulnerable system", highLowNone, "H"},
	"VA": {"Availability, vulnerable system", highLowNone, "H"},
	"SC": {"Confidentiality, subsequent systems", highLowNone, "H"},
	"SI": {"Integrity, subsequent systems", highLowNone, "H"},
	"SA": {"Availability, subsequent systems", highLowNone, "H"},
	"E":  {"Exploit maturity", cvss4Exploit, "A"},
	"CR": {"Confidentiality requirement", requirement, ""},
	"IR": {"Integrity requirement", requirement, ""},
	"AR": {"Availability requirement", requirement, ""},
	"S":  {"Safety", cvss4Safety, "P"},
	"AU": {"Automatable", cvss4Automate, "Y"},
	"R":  {"Recovery", cvss4Recovery, "I"},
	"V":  {"Value density", cvss4Density, ""},
	"RE": {"Response effort", cvss4Effort, ""},
	"U":  {"Provider urgency", cvss4Urgency, "Red"},
}

var cvss2Metrics = map[string]metricSpec{
	"AV": {"Attack vector", map[string]string{"N": "Network", "A": "Adjacent network", "L": "Local"}, "N"},
	"AC": {"Attack complexity", cvss2AC, "L"},
	"Au": {"Authentication", cvss2Auth, "N"},
	"C":  {"Confidentiality", partialImpact, "C"},
	"I":  {"Integrity", partialImpact, "C"},
	"A":  {"Availability", partialImpact, "C"},
}

func metricsFor(version string) map[string]metricSpec {
	switch {
	case version == "":
		return cvss2Metrics
	case strings.HasPrefix(version, "4."):
		return cvss4Metrics
	case strings.HasPrefix(version, "3."):
		return cvss3Metrics
	}
	return nil
}

func lookupMetric(table map[string]metricSpec, key string) (metricSpec, bool) {
	if spec, ok := table[key]; ok {
		return spec, true
	}
	baseKey, modified := strings.CutPrefix(key, modifiedPrefix)
	base, ok := table[baseKey]
	if !ok || !modified {
		return metricSpec{}, false
	}
	return metricSpec{name: "Modified " + strings.ToLower(base.name[:1]) + base.name[1:], values: base.values, severe: base.severe}, true
}

func DescribeVector(vector string) []VectorMetric {
	parts := strings.Split(strings.TrimSpace(vector), "/")
	version, prefixed := strings.CutPrefix(parts[0], vectorPrefix)
	if prefixed {
		parts = parts[1:]
	} else {
		version = ""
	}

	table := metricsFor(version)
	if table == nil || len(parts) == 0 {
		return nil
	}

	metrics := make([]VectorMetric, 0, len(parts))
	for _, part := range parts {
		key, value, ok := strings.Cut(part, ":")
		if !ok || key == "" || value == "" {
			return nil
		}
		if value == notDefinedValue {
			continue
		}

		spec, known := lookupMetric(table, key)
		if !known {
			metrics = append(metrics, VectorMetric{Metric: key, Value: value})
			continue
		}

		name, knownValue := spec.values[value]
		if !knownValue {
			name = value
		}
		metrics = append(metrics, VectorMetric{Metric: spec.name, Value: name, Severe: knownValue && value == spec.severe})
	}

	return metrics
}
