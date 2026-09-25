package intel

type AttackReference struct {
	Source      string `json:"source"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type AttackMitigation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type LogSource struct {
	Name    string `json:"name"`
	Channel string `json:"channel"`
}

type Tunable struct {
	Field       string `json:"field"`
	Description string `json:"description"`
}

type Analytic struct {
	ID          string      `json:"id"`
	Platforms   []string    `json:"platforms"`
	Description string      `json:"description"`
	LogSources  []LogSource `json:"logSources"`
	Tunables    []Tunable   `json:"tunables"`
}

type AttackDetection struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	Analytics []Analytic `json:"analytics"`
}

type Procedure struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

type AttackDetail struct {
	ID          string
	Description string
	References  []AttackReference
	Mitigations []AttackMitigation
	Detections  []AttackDetection
	Procedures  []Procedure
}

func (s *Set) AttackDetail(id string) (AttackDetail, error) {
	row, err := s.lookup(NameAttackDetail, id)
	if err != nil {
		return AttackDetail{}, err
	}

	detail := AttackDetail{ID: row[0], Description: row[1]}
	fields := []jsonField{
		{"references", row[2], &detail.References},
		{"mitigations", row[3], &detail.Mitigations},
		{"detection strategies", row[4], &detail.Detections},
		{"procedure examples", row[5], &detail.Procedures},
	}

	if err := decodeJSONFields(id, fields); err != nil {
		return AttackDetail{}, err
	}

	return detail, nil
}
