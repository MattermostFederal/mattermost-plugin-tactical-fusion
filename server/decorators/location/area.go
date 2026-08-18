package location

import (
	"fmt"
	"math"
	"strings"
)

const (
	alphaNoIO   = "ABCDEFGHJKLMNPQRSTUVWXYZ"
	georefBands = "ABCDEFGHJKLM"
	georefUnits = "ABCDEFGHJKLMNPQ"
	olcBody     = "23456789CFGHJMPQRVWX"
)

type areaCell struct {
	Lat, Lon         float64
	LatSize, LonSize float64
}

func decodeArea(a Area) (areaCell, bool) {
	switch a.Format {
	case FormatGEOREF:
		return georefCell(a.Code)
	case FormatGARS:
		return garsCell(a.Code)
	case FormatPlusCode:
		return olcCell(a.Code)
	default:
		return areaCell{}, false
	}
}

func areaCenter(a Area) (lat, lon float64, ok bool) {
	cell, ok := decodeArea(a)
	if !ok {
		return 0, 0, false
	}
	return cell.Lat + cell.LatSize/2, cell.Lon + cell.LonSize/2, true
}

func areaResolutionDegrees(a Area) float64 {
	cell, ok := decodeArea(a)
	if !ok {
		return 1
	}
	return max(cell.LatSize, cell.LonSize)
}

func georefCell(code string) (areaCell, bool) {
	if len(code) < 4 {
		return areaCell{}, false
	}

	lonZone := strings.IndexByte(alphaNoIO, code[0])
	latBand := strings.IndexByte(georefBands, code[1])
	lonUnit := strings.IndexByte(georefUnits, code[2])
	latUnit := strings.IndexByte(georefUnits, code[3])
	if lonZone < 0 || latBand < 0 || lonUnit < 0 || latUnit < 0 {
		return areaCell{}, false
	}

	cell := areaCell{
		Lat:     float64(-90 + 15*latBand + latUnit),
		Lon:     float64(-180 + 15*lonZone + lonUnit),
		LatSize: 1,
		LonSize: 1,
	}

	digits := code[4:]
	if digits == "" {
		return cell, true
	}

	if len(digits)%2 != 0 {
		return areaCell{}, false
	}

	perAxis := len(digits) / 2
	if perAxis < 2 || perAxis > 4 {
		return areaCell{}, false
	}

	unitsPerDegree := 60 * pow10(perAxis-2)

	lonMinutes := atoi(digits[:perAxis])
	latMinutes := atoi(digits[perAxis:])
	if lonMinutes < 0 || lonMinutes >= unitsPerDegree ||
		latMinutes < 0 || latMinutes >= unitsPerDegree {
		return areaCell{}, false
	}

	size := 1 / float64(unitsPerDegree)
	cell.Lon += float64(lonMinutes) * size
	cell.Lat += float64(latMinutes) * size
	cell.LatSize, cell.LonSize = size, size

	return cell, true
}

func garsCell(code string) (areaCell, bool) {
	if len(code) < 5 || len(code) > 7 {
		return areaCell{}, false
	}

	for i := range 3 {
		if code[i] < '0' || code[i] > '9' {
			return areaCell{}, false
		}
	}

	lonBand := atoi(code[:3])
	if lonBand < 1 || lonBand > 720 {
		return areaCell{}, false
	}

	high := strings.IndexByte(alphaNoIO, code[3])
	low := strings.IndexByte(alphaNoIO, code[4])
	if high < 0 || low < 0 {
		return areaCell{}, false
	}

	latBand := high*len(alphaNoIO) + low
	if latBand > 359 {
		return areaCell{}, false
	}

	lonMinutes := (lonBand-1)*30 - 180*60
	latMinutes := latBand*30 - 90*60
	sizeMinutes := 30

	if len(code) >= 6 {
		quadrant := int(code[5] - '0')
		if quadrant < 1 || quadrant > 4 {
			return areaCell{}, false
		}
		if quadrant == 2 || quadrant == 4 {
			lonMinutes += 15
		}
		if quadrant <= 2 {
			latMinutes += 15
		}
		sizeMinutes = 15
	}

	if len(code) == 7 {
		keypad := int(code[6] - '0')
		if keypad < 1 || keypad > 9 {
			return areaCell{}, false
		}
		lonMinutes += ((keypad - 1) % 3) * 5
		latMinutes += (2 - (keypad-1)/3) * 5
		sizeMinutes = 5
	}

	return areaCell{
		Lat:     float64(latMinutes) / 60,
		Lon:     float64(lonMinutes) / 60,
		LatSize: float64(sizeMinutes) / 60,
		LonSize: float64(sizeMinutes) / 60,
	}, true
}

var olcPairUnits = [5]int64{160000, 8000, 400, 20, 1}

const (
	olcUnitsPerDegree = 8000
	olcMaxFirstLat    = 8
	olcMaxFirstLon    = 17
	olcSeparatorAt    = 8
	olcMaxPadding     = 4

	olcGridRows    = 5
	olcGridColumns = 4
)

func olcCell(code string) (areaCell, bool) {
	if strings.Count(code, "+") != 1 || strings.IndexByte(code, '+') != olcSeparatorAt {
		return areaCell{}, false
	}

	head, tail := code[:olcSeparatorAt], code[olcSeparatorAt+1:]

	padding := 0
	for padding < len(head) && head[len(head)-1-padding] == '0' {
		padding++
	}
	if padding > 0 && (tail != "" || padding%2 != 0 || padding > olcMaxPadding) {
		return areaCell{}, false
	}

	significant := head[:len(head)-padding] + tail

	switch n := len(significant); {
	case n == 4, n == 6, n == 8, n >= 10 && n <= 15:
	default:
		return areaCell{}, false
	}

	pairs := min(len(significant)/2, len(olcPairUnits))

	var latUnits, lonUnits, pairCellUnits int64
	for i := range pairs {
		latIndex := int64(strings.IndexByte(olcBody, significant[2*i]))
		lonIndex := int64(strings.IndexByte(olcBody, significant[2*i+1]))
		if latIndex < 0 || lonIndex < 0 {
			return areaCell{}, false
		}
		if i == 0 && (latIndex > olcMaxFirstLat || lonIndex > olcMaxFirstLon) {
			return areaCell{}, false
		}

		latUnits += latIndex * olcPairUnits[i]
		lonUnits += lonIndex * olcPairUnits[i]
		pairCellUnits = olcPairUnits[i]
	}

	latDenominator, lonDenominator := int64(olcUnitsPerDegree), int64(olcUnitsPerDegree)
	latCellUnits, lonCellUnits := pairCellUnits, pairCellUnits

	for i := 2 * pairs; i < len(significant); i++ {
		index := int64(strings.IndexByte(olcBody, significant[i]))
		if index < 0 {
			return areaCell{}, false
		}

		latUnits = latUnits*olcGridRows + (index/olcGridColumns)*latCellUnits
		lonUnits = lonUnits*olcGridColumns + (index%olcGridColumns)*lonCellUnits
		latDenominator *= olcGridRows
		lonDenominator *= olcGridColumns
	}

	return areaCell{
		Lat:     float64(latUnits)/float64(latDenominator) - 90,
		Lon:     float64(lonUnits)/float64(lonDenominator) - 180,
		LatSize: float64(latCellUnits) / float64(latDenominator),
		LonSize: float64(lonCellUnits) / float64(lonDenominator),
	}, true
}

type ladderStep struct {
	value int
	size  float64
}

func ladderValue(steps []ladderStep, degrees float64) int {
	for _, step := range steps {
		if step.size <= degrees {
			return step.value
		}
	}
	return steps[len(steps)-1].value
}

var georefLadder = []ladderStep{
	{0, 1},
	{2, 1.0 / 60},
	{3, 1.0 / 600},
	{4, 1.0 / 6000},
}

var garsLadder = []ladderStep{
	{30, 30.0 / 60},
	{15, 15.0 / 60},
	{5, 5.0 / 60},
}

var olcLadder = []ladderStep{
	{4, 1},
	{6, 0.05},
	{8, 0.0025},
	{10, 0.000125},
	{11, 0.000125 / 4},
	{12, 0.000125 / 16},
	{13, 0.000125 / 64},
	{14, 0.000125 / 256},
	{15, 0.000125 / 1024},
}

func georefDigitsFor(degrees float64) int   { return ladderValue(georefLadder, degrees) }
func garsMinutesFor(degrees float64) int    { return ladderValue(garsLadder, degrees) }
func plusCodeLengthFor(degrees float64) int { return ladderValue(olcLadder, degrees) }

func georefAt(lat, lon float64, digits int) string {
	unitsPerDegree := 1
	if digits >= 2 {
		unitsPerDegree = 60 * pow10(digits-2)
	}

	latUnits := cellIndex(lat+90, unitsPerDegree, 180*unitsPerDegree)
	lonUnits := cellIndex(lon+180, unitsPerDegree, 360*unitsPerDegree)

	latDegrees, latMinutes := latUnits/unitsPerDegree, latUnits%unitsPerDegree
	lonDegrees, lonMinutes := lonUnits/unitsPerDegree, lonUnits%unitsPerDegree

	var b strings.Builder
	b.WriteByte(alphaNoIO[lonDegrees/15])
	b.WriteByte(georefBands[latDegrees/15])
	b.WriteByte(georefUnits[lonDegrees%15])
	b.WriteByte(georefUnits[latDegrees%15])

	if digits >= 2 {
		fmt.Fprintf(&b, "%0*d%0*d", digits, lonMinutes, digits, latMinutes)
	}

	return b.String()
}

func garsAt(lat, lon float64, sizeMinutes int) string {
	latMinutes := cellIndex(lat+90, 60, 180*60)
	lonMinutes := cellIndex(lon+180, 60, 360*60)

	var b strings.Builder
	fmt.Fprintf(&b, "%03d", lonMinutes/30+1)

	latBand := latMinutes / 30
	b.WriteByte(alphaNoIO[latBand/len(alphaNoIO)])
	b.WriteByte(alphaNoIO[latBand%len(alphaNoIO)])

	if sizeMinutes > 15 {
		return b.String()
	}

	quadrant := 3 + (lonMinutes%30)/15
	if (latMinutes%30)/15 == 1 {
		quadrant -= 2
	}
	b.WriteByte(byte('0' + quadrant))

	if sizeMinutes > 5 {
		return b.String()
	}

	column := (lonMinutes % 15) / 5
	rowFromNorth := 2 - (latMinutes%15)/5
	b.WriteByte(byte('0' + rowFromNorth*3 + column + 1))

	return b.String()
}

func plusCodeAt(lat, lon float64, length int) string {
	length = clampOLCLength(length)

	pairs := min(length, 2*len(olcPairUnits)) / 2
	gridChars := max(length-2*len(olcPairUnits), 0)

	latDenominator, lonDenominator := int64(olcUnitsPerDegree), int64(olcUnitsPerDegree)
	for range gridChars {
		latDenominator *= olcGridRows
		lonDenominator *= olcGridColumns
	}

	latUnits := cellIndex64(lat+90, latDenominator, 180*latDenominator)
	lonUnits := cellIndex64(lon+180, lonDenominator, 360*lonDenominator)

	out := make([]byte, length)

	for i := gridChars; i > 0; i-- {
		grid := (latUnits%olcGridRows)*olcGridColumns + lonUnits%olcGridColumns
		out[2*len(olcPairUnits)+i-1] = olcBody[grid]
		latUnits /= olcGridRows
		lonUnits /= olcGridColumns
	}

	for i := len(olcPairUnits); i > 0; i-- {
		if i <= pairs {
			out[2*(i-1)] = olcBody[latUnits%20]
			out[2*(i-1)+1] = olcBody[lonUnits%20]
		}
		latUnits /= 20
		lonUnits /= 20
	}

	var b strings.Builder
	b.Write(out[:min(length, olcSeparatorAt)])
	b.WriteString(strings.Repeat("0", max(olcSeparatorAt-length, 0)))
	b.WriteByte('+')
	if length > olcSeparatorAt {
		b.Write(out[olcSeparatorAt:])
	}

	return b.String()
}

func clampOLCLength(length int) int {
	if length >= 10 {
		return min(length, 15)
	}
	if length < 4 {
		return 4
	}
	return length - length%2
}

const cellSnap = 1e6

func snappedCell(shifted float64, unitsPerDegree float64) float64 {
	return math.Floor(math.Round(shifted*unitsPerDegree*cellSnap) / cellSnap)
}

func cellIndex(shifted float64, unitsPerDegree, limit int) int {
	index := int(snappedCell(shifted, float64(unitsPerDegree)))
	return min(max(index, 0), limit-1)
}

func cellIndex64(shifted float64, unitsPerDegree, limit int64) int64 {
	index := int64(snappedCell(shifted, float64(unitsPerDegree)))
	return min(max(index, 0), limit-1)
}

func pow10(n int) int {
	out := 1
	for range n {
		out *= 10
	}
	return out
}
