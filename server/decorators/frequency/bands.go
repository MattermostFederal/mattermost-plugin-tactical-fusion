package frequency

import "strconv"

type Band struct {
	LowKHz  int
	HighKHz int
	Name    string
}

var Bands = []Band{
	{2_000, 30_000, "HF aeronautical"},
	{108_000, 117_975, "VHF navigation"},
	{118_000, 136_975, "VHF air band"},
	{156_000, 162_025, "VHF marine"},
	{225_000, 400_000, "UHF military air band"},
	{406_000, 406_100, "Distress beacons"},
}

type Allocation struct {
	KHz int
	Use string
}

var Allocations = []Allocation{
	{121_500, "Aeronautical emergency"},
	{123_100, "Search and rescue on-scene"},
	{122_750, "Air-to-air, fixed wing"},
	{123_450, "Air-to-air"},
	{243_000, "UHF military emergency"},
	{156_800, "Marine channel 16, distress and calling"},
	{406_000, "Distress beacon, COSPAS-SARSAT"},
}

const OutsideBands = "Outside the aviation bands this plugin names"

type Details struct {
	Token   string
	MHz     string
	KHz     string
	Band    string
	Channel string
	Use     string
}

func Describe(f Frequency) Details {
	return Details{
		Token:   f.Token,
		MHz:     MHzText(f.KHz),
		KHz:     strconv.Itoa(f.KHz),
		Band:    BandOf(f.KHz),
		Channel: ChannelOf(f.KHz),
		Use:     UseOf(f.KHz),
	}
}

func MHzText(khz int) string {
	return strconv.Itoa(khz/1000) + "." + pad3(khz%1000)
}

func pad3(n int) string {
	s := strconv.Itoa(n)
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

func BandOf(khz int) string {
	for _, band := range Bands {
		if khz >= band.LowKHz && khz <= band.HighKHz {
			return band.Name
		}
	}
	return OutsideBands
}

func ChannelOf(khz int) string {
	if khz < 118_000 || khz > 136_975 {
		return ""
	}
	if khz%25 == 0 {
		return "25 kHz channel"
	}
	return "8.33 kHz channel"
}

func UseOf(khz int) string {
	for _, allocation := range Allocations {
		if allocation.KHz == khz {
			return allocation.Use
		}
	}
	return ""
}
