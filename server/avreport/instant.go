package avreport

import (
	"strconv"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

func resolveGroup(day, hour, minute string, ref time.Time) (time.Time, bool) {
	d, _ := strconv.Atoi(day)
	h, _ := strconv.Atoi(hour)
	m, _ := strconv.Atoi(minute)
	return dtg.ResolveDayTime(d, h, m, ref)
}

func fullDate(raw string) (time.Time, bool) {
	if len(raw) != 10 || !isDigits(raw) {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(raw[0:2])
	if err != nil {
		return time.Time{}, false
	}
	month, err := strconv.Atoi(raw[2:4])
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(raw[4:6])
	if err != nil || day < 1 || day > 31 {
		return time.Time{}, false
	}
	hour, err := strconv.Atoi(raw[6:8])
	if err != nil || hour > 23 {
		return time.Time{}, false
	}
	minute, err := strconv.Atoi(raw[8:10])
	if err != nil || minute > 59 {
		return time.Time{}, false
	}
	t := time.Date(2000+year, time.Month(month), day, hour, minute, 0, 0, time.UTC)
	if t.Day() != day {
		return time.Time{}, false
	}
	return t, true
}

func zuluText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("02 Jan 2006 15:04") + "Z"
}

func dayHourText(day, hour string) string {
	return "day " + day + " at " + hour + ":00Z"
}

func dayHourMinuteText(day, hour, minute string) string {
	return "day " + day + " at " + hour + ":" + minute + "Z"
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
