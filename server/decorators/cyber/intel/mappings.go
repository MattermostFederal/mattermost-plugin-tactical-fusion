package intel

type AttackPattern struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Abstraction string `json:"abstraction"`
	Severity    string `json:"severity"`
	Likelihood  string `json:"likelihood"`
	Summary     string `json:"summary"`
}

func (s *Set) AttackPatterns(key string) ([]AttackPattern, error) {
	row, err := s.lookup(NameCAPEC, key)
	if err != nil {
		return nil, err
	}

	var patterns []AttackPattern
	if err := decodeJSONFields(key, []jsonField{{"attack patterns", row[1], &patterns}}); err != nil {
		return nil, err
	}
	return patterns, nil
}

type Mapping struct {
	ID    string   `json:"id"`
	Types []string `json:"types"`
}

func (s *Set) ATTACKMappings(key string) ([]Mapping, error) {
	row, err := s.lookup(NameCVEAttack, key)
	if err != nil {
		return nil, err
	}

	var mappings []Mapping
	if err := decodeJSONFields(key, []jsonField{{"ATT&CK mappings", row[1], &mappings}}); err != nil {
		return nil, err
	}
	return mappings, nil
}
