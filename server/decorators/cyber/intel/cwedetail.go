package intel

import (
	"encoding/json"
	"fmt"
)

type Consequence struct {
	Scopes     []string `json:"scopes"`
	Impacts    []string `json:"impacts"`
	Likelihood string   `json:"likelihood"`
	Note       string   `json:"note"`
}

type Mitigation struct {
	Phase         string `json:"phase"`
	Strategy      string `json:"strategy"`
	Description   string `json:"description"`
	Effectiveness string `json:"effectiveness"`
}

type Detection struct {
	Method        string `json:"method"`
	Description   string `json:"description"`
	Effectiveness string `json:"effectiveness"`
}

type Example struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type CWEDetail struct {
	ID           string
	Description  string
	Extended     string
	Consequences []Consequence
	Mitigations  []Mitigation
	Detections   []Detection
	Examples     []Example
}

func (s *Set) CWEDetail(id string) (CWEDetail, error) {
	row, err := s.lookup(NameCWEDetail, id)
	if err != nil {
		return CWEDetail{}, err
	}

	detail := CWEDetail{ID: row[0], Description: row[1], Extended: row[2]}
	fields := []struct {
		name string
		raw  string
		into any
	}{
		{"consequences", row[3], &detail.Consequences},
		{"mitigations", row[4], &detail.Mitigations},
		{"detection methods", row[5], &detail.Detections},
		{"observed examples", row[6], &detail.Examples},
	}

	for _, field := range fields {
		if field.raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(field.raw), field.into); err != nil {
			return CWEDetail{}, fmt.Errorf("the %s of %s are not the shape this build reads: %w", field.name, id, err)
		}
	}

	return detail, nil
}
