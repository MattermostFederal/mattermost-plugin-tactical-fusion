package airport

import (
	"encoding/json"
)

const MapKind = "airport"

type mapCoordinate struct {
	Format string `json:"format"`
	Value  string `json:"value"`
	Region string `json:"region"`
}

type mapEnd struct {
	Format string `json:"format"`
	Value  string `json:"value"`
}

type mapRunway struct {
	Designation string    `json:"designation"`
	Ends        [2]mapEnd `json:"ends"`
}

type mapBlob struct {
	Ident      string        `json:"ident"`
	Name       string        `json:"name"`
	Coordinate mapCoordinate `json:"coordinate"`
	Runways    []mapRunway   `json:"runways"`
}

func MapBlob(ident string) (string, bool) {
	d, ok := Describe(ident)
	if !ok || !d.HasPosition {
		return "", false
	}

	blob := mapBlob{
		Ident:      d.Ident,
		Name:       d.Name,
		Coordinate: mapCoordinate{Format: d.Format, Value: d.Token, Region: d.Region},
		Runways:    []mapRunway{},
	}
	for _, r := range d.Runways {
		if r.Ends == nil {
			continue
		}
		blob.Runways = append(blob.Runways, mapRunway{
			Designation: r.Designation,
			Ends: [2]mapEnd{
				{Format: r.Ends[0].Format, Value: r.Ends[0].Token},
				{Format: r.Ends[1].Format, Value: r.Ends[1].Token},
			},
		})
	}

	encoded, err := json.Marshal(blob)
	if err != nil {
		return "", false
	}
	return string(encoded), true
}
