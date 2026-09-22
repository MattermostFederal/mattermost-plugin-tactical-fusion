package avreport

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/airport"
)

const (
	KindMETAR = "METAR"
	KindSPECI = "SPECI"
	KindTAF   = "TAF"
	KindNOTAM = "NOTAM"
)

var Kinds = []string{KindMETAR, KindSPECI, KindTAF, KindNOTAM}

var intensities = map[byte]string{'-': "light ", '+': "heavy "}

var descriptors = map[string]string{
	"MI": "shallow", "PR": "partial", "BC": "patches of", "DR": "low drifting",
	"BL": "blowing", "SH": "showers of", "TS": "thunderstorm with", "FZ": "freezing",
}

var phenomena = map[string]string{
	"DZ": "drizzle", "RA": "rain", "SN": "snow", "SG": "snow grains", "IC": "ice crystals",
	"PL": "ice pellets", "GR": "hail", "GS": "small hail", "UP": "unknown precipitation",
	"BR": "mist", "FG": "fog", "FU": "smoke", "VA": "volcanic ash", "DU": "widespread dust",
	"SA": "sand", "HZ": "haze", "PY": "spray", "PO": "dust whirls", "SQ": "squalls",
	"FC": "funnel cloud", "SS": "sandstorm", "DS": "duststorm",
}

var skyCover = map[string]string{
	"FEW": "few clouds", "SCT": "scattered clouds", "BKN": "broken clouds", "OVC": "overcast",
}

var skyClear = map[string]string{
	"SKC": "sky clear", "CLR": "clear below 12,000 ft", "NSC": "no significant cloud", "NCD": "no cloud detected",
}

var cloudTypes = map[string]string{"CB": "cumulonimbus", "TCU": "towering cumulus"}

var windPattern = regexp.MustCompile(`^(\d{3}|VRB|///)(\d{2,3}|//)(?:G(\d{2,3}))?(KT|MPS|KMH)$`)

var windVariablePattern = regexp.MustCompile(`^(\d{3})V(\d{3})$`)

var visibilityMetersPattern = regexp.MustCompile(`^(\d{4})(NDV)?$`)

var visibilityMilesPattern = regexp.MustCompile(`^([MP])?(\d{1,2})?(?: ?(\d)/(\d{1,2}))?SM$`)

var rvrPattern = regexp.MustCompile(`^R(\d{2}[LRC]?)/([MP]?)(\d{4})(?:V([MP]?)(\d{4}))?(FT)?/?([UDN])?$`)

var weatherPattern = regexp.MustCompile(`^([-+])?(VC)?(MI|PR|BC|DR|BL|SH|TS|FZ)?((?:DZ|RA|SN|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PY|PO|SQ|FC|SS|DS){1,3})$`)

var skyPattern = regexp.MustCompile(`^(FEW|SCT|BKN|OVC)(\d{3}|///)(CB|TCU|///)?$`)

var verticalVisibilityPattern = regexp.MustCompile(`^VV(\d{3}|///)$`)

var temperaturePattern = regexp.MustCompile(`^(M?\d{2}|//)/(M?\d{2}|//)?$`)

var altimeterPattern = regexp.MustCompile(`^([AQ])(\d{4})$`)

var windShearPattern = regexp.MustCompile(`^WS(\d{3})/(\d{3})(\d{2,3})KT$`)

var tafValidityPattern = regexp.MustCompile(`^(\d{2})(\d{2})/(\d{2})(\d{2})$`)

var tafFromPattern = regexp.MustCompile(`^FM(\d{2})(\d{2})(\d{2})$`)

var tafProbPattern = regexp.MustCompile(`^PROB(30|40)$`)

var tafExtremePattern = regexp.MustCompile(`^T([XN])(M?\d{2})/(\d{2})(\d{2})Z$`)

var slpPattern = regexp.MustCompile(`^SLP(\d{3})$`)

var preciseTemperaturePattern = regexp.MustCompile(`^T([01])(\d{3})(?:([01])(\d{3}))?$`)

var peakWindPattern = regexp.MustCompile(`^(\d{3})(\d{2,3})/(\d{2})?(\d{2})$`)

var precipitationEventPattern = regexp.MustCompile(`^((?:[-+]?(?:MI|PR|BC|DR|BL|SH|TS|FZ)?(?:DZ|RA|SN|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PY|PO|SQ|FC|SS|DS)){1,2})((?:[BE]\d{2,4}){1,2})$`)

var hourlyPrecipitationPattern = regexp.MustCompile(`^P(\d{4})$`)

var remarkFlags = map[string]string{
	"AO1":     "automated station without a precipitation discriminator",
	"AO2":     "automated station with a precipitation discriminator",
	"$":       "maintenance is needed on the station",
	"PRESRR":  "pressure rising rapidly",
	"PRESFR":  "pressure falling rapidly",
	"TSNO":    "thunderstorm information not available",
	"RVRNO":   "runway visual range not available",
	"SLPNO":   "sea level pressure not available",
	"PNO":     "precipitation amount not available",
	"FZRANO":  "freezing rain information not available",
	"PWINO":   "present weather sensor not available",
	"VISNO":   "visibility sensor not available",
	"CHINO":   "ceiling sensor not available",
	"NOSPECI": "no SPECI reports are issued",
}

func compass(degrees string) string {
	return degrees + "°"
}

func windText(m []string) string {
	direction, speed, gust, unit := m[1], m[2], m[3], m[4]
	unitText := map[string]string{"KT": "kt", "MPS": "m/s", "KMH": "km/h"}[unit]

	if direction == "///" || speed == "//" {
		return "not reported"
	}
	if direction == "000" && speed == "00" {
		return "calm"
	}

	speedValue, _ := strconv.Atoi(speed)
	text := ""
	if direction == "VRB" {
		text = "variable at " + strconv.Itoa(speedValue) + " " + unitText
	} else {
		text = compass(direction) + " at " + strconv.Itoa(speedValue) + " " + unitText
	}
	if gust != "" {
		gustValue, _ := strconv.Atoi(gust)
		text += ", gusting " + strconv.Itoa(gustValue) + " " + unitText
	}
	return text
}

func visibilityMetersText(m []string) string {
	if m[1] == "9999" {
		return "10 km or more"
	}
	meters, _ := strconv.Atoi(m[1])
	if meters == 0 {
		return "less than 50 m"
	}
	text := airport.WithThousands(meters) + " m"
	if m[2] != "" {
		text += ", no directional variation"
	}
	return text
}

func visibilityMilesText(m []string) string {
	modifier, whole, numerator, denominator := m[1], m[2], m[3], m[4]

	var value string
	switch {
	case whole != "" && numerator != "":
		value = whole + " " + numerator + "/" + denominator
	case numerator != "":
		value = numerator + "/" + denominator
	default:
		value = whole
	}

	unit := " statute miles"
	if value == "1" {
		unit = " statute mile"
	}

	switch modifier {
	case "M":
		return "less than " + value + unit
	case "P":
		return "more than " + value + unit
	}
	return value + unit
}

func weatherText(m []string) string {
	var b strings.Builder
	if m[1] != "" {
		b.WriteString(intensities[m[1][0]])
	}
	if m[2] != "" {
		b.WriteString("in the vicinity, ")
	}
	if m[3] != "" {
		b.WriteString(descriptors[m[3]] + " ")
	}
	codes := m[4]
	parts := make([]string, 0, len(codes)/2)
	for i := 0; i+2 <= len(codes); i += 2 {
		parts = append(parts, phenomena[codes[i:i+2]])
	}
	b.WriteString(strings.Join(parts, " and "))
	return b.String()
}

func skyText(m []string) string {
	cover := skyCover[m[1]]
	if m[2] == "///" {
		return cover + " at an unknown height"
	}
	hundreds, _ := strconv.Atoi(m[2])
	text := cover + " at " + airport.WithThousands(hundreds*100) + " ft"
	if kind, ok := cloudTypes[m[3]]; ok {
		text += ", " + kind
	}
	return text
}

func temperatureValue(raw string) (string, bool) {
	if raw == "//" || raw == "" {
		return "", false
	}
	negative := strings.HasPrefix(raw, "M")
	digits := strings.TrimPrefix(raw, "M")
	value, err := strconv.Atoi(digits)
	if err != nil {
		return "", false
	}
	if negative {
		value = -value
	}
	return strconv.Itoa(value) + "°C", true
}

func altimeterText(m []string) string {
	if m[1] == "A" {
		return m[2][:2] + "." + m[2][2:] + " inHg"
	}
	hpa, _ := strconv.Atoi(m[2])
	return strconv.Itoa(hpa) + " hPa"
}

func rvrText(m []string) string {
	runway, lowMod, low, highMod, high, feet, trend := m[1], m[2], m[3], m[4], m[5], m[6], m[7]
	unit := " m"
	if feet != "" {
		unit = " ft"
	}
	modifierText := map[string]string{"M": "less than ", "P": "more than ", "": ""}

	lowValue, _ := strconv.Atoi(low)
	text := "runway " + runway + ": " + modifierText[lowMod] + airport.WithThousands(lowValue) + unit
	if high != "" {
		highValue, _ := strconv.Atoi(high)
		text = "runway " + runway + ": " + modifierText[lowMod] + airport.WithThousands(lowValue) +
			" to " + modifierText[highMod] + airport.WithThousands(highValue) + unit
	}
	switch trend {
	case "U":
		text += ", improving"
	case "D":
		text += ", worsening"
	case "N":
		text += ", no change"
	}
	return text
}

func slpText(m []string) string {
	tenths, _ := strconv.Atoi(m[1])
	hpa := 1000.0 + float64(tenths)/10
	if tenths >= 500 {
		hpa = 900.0 + float64(tenths)/10
	}
	return strconv.FormatFloat(hpa, 'f', 1, 64) + " hPa"
}

func preciseTemperatureText(m []string) string {
	value := func(sign, digits string) string {
		tenths, _ := strconv.Atoi(digits)
		text := strconv.FormatFloat(float64(tenths)/10, 'f', 1, 64) + "°C"
		if sign == "1" {
			return "-" + text
		}
		return text
	}
	text := value(m[1], m[2])
	if m[3] != "" {
		text += ", dew point " + value(m[3], m[4])
	}
	return text
}
