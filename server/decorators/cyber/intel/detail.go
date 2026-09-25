package intel

import (
	"encoding/json"
	"fmt"
)

type WeaknessSource struct {
	Source string   `json:"source"`
	CWE    []string `json:"cwe"`
}

type CPEMatch struct {
	CPE     string `json:"cpe"`
	From    string `json:"from"`
	After   string `json:"after"`
	Through string `json:"through"`
	Before  string `json:"before"`
}

type Configuration struct {
	Vulnerable []CPEMatch `json:"vulnerable"`
	On         []CPEMatch `json:"on"`
}

type VersionChange struct {
	At     string `json:"at"`
	Status string `json:"status"`
}

type AffectedVersion struct {
	Version         string          `json:"version"`
	Status          string          `json:"status"`
	LessThan        string          `json:"lessThan"`
	LessThanOrEqual string          `json:"lessThanOrEqual"`
	Changes         []VersionChange `json:"changes"`
}

type AffectedProduct struct {
	Vendor        string            `json:"vendor"`
	Product       string            `json:"product"`
	DefaultStatus string            `json:"defaultStatus"`
	Versions      []AffectedVersion `json:"versions"`
}

type Reference struct {
	URL  string   `json:"url"`
	Tags []string `json:"tags"`
}

type CVEDetail struct {
	ID             string
	Weaknesses     []WeaknessSource
	Configurations []Configuration
	Affected       []AffectedProduct
	References     []Reference
}

func (s *Set) CVEDetail(id string) (CVEDetail, error) {
	row, err := s.lookup(NameCVEDetail, id)
	if err != nil {
		return CVEDetail{}, err
	}

	detail := CVEDetail{ID: row[0]}
	fields := []jsonField{
		{"weaknesses", row[1], &detail.Weaknesses},
		{"configurations", row[2], &detail.Configurations},
		{"affected", row[3], &detail.Affected},
		{"references", row[4], &detail.References},
	}

	if err := decodeJSONFields(id, fields); err != nil {
		return CVEDetail{}, err
	}

	return detail, nil
}

type jsonField struct {
	name string
	raw  string
	into any
}

func decodeJSONFields(id string, fields []jsonField) error {
	for _, field := range fields {
		if field.raw == "" {
			continue
		}
		if err := json.Unmarshal([]byte(field.raw), field.into); err != nil {
			return fmt.Errorf("the %s of %s are not the shape this build reads: %w", field.name, id, err)
		}
	}
	return nil
}
