package main

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/mattermost/mattermost/server/public/plugin"
	"github.com/pkg/errors"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/location"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// preferencesVersion is stamped on every blob written.
//
// Nothing reads it yet. It is here so a later change of shape can tell an old
// blob from a new one, which is far cheaper to add now than to retrofit onto
// data already sitting in the KV store.
const preferencesVersion = 1

// ZoneSelection is one row a reader chose.
//
// Name carries the base they picked it by, because several installations share
// a zone and somebody at Stuttgart wants to see "Stuttgart" rather than the
// Ramstein row that keeps the same clock. It is a label and nothing more: this
// server never resolves it, and the catalog it comes from lives only in the
// webapp, which is what keeps a base list from having to be maintained twice.
type ZoneSelection struct {
	IANA string `json:"iana"`
	Name string `json:"name,omitempty"`
}

// UnmarshalJSON also accepts a bare identifier.
//
// Blobs written before names existed hold plain strings. They mean an unnamed
// zone, which is exactly what they were, so they are read rather than discarded.
func (z *ZoneSelection) UnmarshalJSON(data []byte) error {
	var bare string
	if err := json.Unmarshal(data, &bare); err == nil {
		z.IANA = bare
		z.Name = ""
		return nil
	}

	// A named type, so unmarshalling the object form does not call this method
	// again and recurse forever.
	type selection ZoneSelection
	var decoded selection
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}

	*z = ZoneSelection(decoded)

	return nil
}

// DTGPreferences is one reader's view of a date-time group.
//
// Every field is optional, and a zero value means "use the default". An absent,
// empty or half-filled blob is therefore a valid one, which is what lets
// "Restore defaults" be a delete rather than a write of today's defaults: a
// reader who never chose keeps tracking whatever the defaults become.
type DTGPreferences struct {
	// Zones are the chosen rows, in no meaningful order: the panel sorts them
	// by offset. Empty means the built-in table in
	// server/decorators/dtg/zones.go.
	Zones []ZoneSelection `json:"zones"`

	// UrgentWithinMinutes is how close a DTG has to be before the countdown
	// flashes. Zero means the built-in threshold.
	UrgentWithinMinutes int `json:"urgent_within_minutes"`
}

// LocationPreferences is one reader's view of a coordinate.
//
// Every field is optional and a zero value means "use the default", the same
// way DTGPreferences works.
type LocationPreferences struct {
	// HiddenRows are the rows to leave out of the panel, by id. Empty means
	// every row, which is the default.
	//
	// The HIDDEN rows rather than the shown ones, and that direction is the
	// whole design. Empty then means "all of them", so a reader who has never
	// chosen is stored as nothing at all, which is what lets "Restore defaults"
	// be a delete. It also decides what happens when a row is ADDED in a later
	// version: stored this way a new row appears for everybody, including
	// readers who customized, which is the same promise the DTG defaults make.
	// Stored the other way round it would be invisible to exactly the readers
	// who cared enough to choose.
	HiddenRows []string `json:"hidden_rows"`
}

// UserPreferences is the whole per-user blob.
//
// Decorators get a key of their own rather than a flat namespace, so a second
// decorator can add settings without migrating anybody's stored blob.
type UserPreferences struct {
	Version  int                 `json:"version"`
	DTG      DTGPreferences      `json:"dtg"`
	Location LocationPreferences `json:"location"`
}

// clone returns a copy that shares no slice with the original.
//
// The cache hands the same value to every caller, so without this a caller that
// appended to Zones would be editing what the next reader gets.
func (p UserPreferences) clone() UserPreferences {
	if p.DTG.Zones != nil {
		p.DTG.Zones = append([]ZoneSelection(nil), p.DTG.Zones...)
	}
	if p.Location.HiddenRows != nil {
		p.Location.HiddenRows = append([]string(nil), p.Location.HiddenRows...)
	}
	return p
}

const (
	// maxZones is a ceiling, not a considered limit. The point is that a
	// crafted request cannot make the panel render an unbounded table.
	maxZones = 25

	// maxZoneIDLength bounds the work LoadLocation is asked to do before it
	// gets a chance to reject anything. Real identifiers are far shorter.
	maxZoneIDLength = 64

	// maxZoneNameLength bounds a row label. The longest real installation name
	// is nowhere near this.
	maxZoneNameLength = 64

	minUrgentWithinMinutes = 1
	maxUrgentWithinMinutes = 24 * 60
)

// zoneIDRe is the character set of an IANA identifier.
//
// LoadLocation already refuses absolute paths and anything containing "..", so
// this is defense in depth rather than the guard itself. It also keeps a
// nonsense value out of the logs and the KV store.
var zoneIDRe = regexp.MustCompile(`^[A-Za-z0-9_+/-]+$`)

// validate normalizes a submitted blob and rejects one that cannot be honored.
//
// Rejecting rather than silently dropping: these values come from the reader's
// own browser, so a value this server will not store is a bug worth surfacing,
// not something to quietly discard and report success for.
func (p *UserPreferences) validate() error {
	p.Version = preferencesVersion

	// Trimmed and deduplicated first, then checked in two passes.
	//
	// The count has to come after the dedup, because two entries that collapse
	// were never two rows.
	zones := make([]ZoneSelection, 0, len(p.DTG.Zones))
	seen := make(map[ZoneSelection]bool, len(p.DTG.Zones))
	for _, zone := range p.DTG.Zones {
		zone.IANA = strings.TrimSpace(zone.IANA)
		zone.Name = strings.TrimSpace(zone.Name)

		// Deduplicated on the pair, not on the zone: two bases sharing a zone
		// are two different rows, and collapsing them would silently drop one.
		if zone.IANA == "" || seen[zone] {
			continue
		}
		seen[zone] = true
		zones = append(zones, zone)
	}

	// Shape before size, and both before the timezone database.
	//
	// A malformed identifier is reported ahead of the count because it names
	// the thing the reader got wrong, where "too many" only says to remove
	// something. Neither check resolves a zone, so a blob that fails either
	// costs no lookups at all: that is the whole reason validZoneID is split
	// from the LoadLocation behind it.
	for _, zone := range zones {
		if err := validZoneShape(zone.IANA); err != nil {
			return err
		}
		if err := validZoneName(zone.Name); err != nil {
			return err
		}
	}

	if len(zones) > maxZones {
		return errcode.Errorf(errcode.PreferencesTooManyZones,
			"at most %d timezones may be selected", maxZones)
	}

	for _, zone := range zones {
		if err := validZoneID(zone.IANA); err != nil {
			return err
		}
	}

	p.DTG.Zones = zones

	if m := p.DTG.UrgentWithinMinutes; m != 0 && (m < minUrgentWithinMinutes || m > maxUrgentWithinMinutes) {
		return errcode.Errorf(errcode.PreferencesThresholdOutOfRange,
			"the flash threshold must be between %d and %d minutes",
			minUrgentWithinMinutes, maxUrgentWithinMinutes)
	}

	rows, err := validHiddenRows(p.Location.HiddenRows)
	if err != nil {
		return err
	}
	p.Location.HiddenRows = rows

	return nil
}

// validHiddenRows normalizes a hidden-row selection.
//
// Deduplicated, then checked against the rows this build actually renders. An
// unknown id is REFUSED rather than dropped, for the same reason a bad timezone
// is: it can only come from a hand-written request or a bug, and quietly
// storing something that will never do anything reports success for a setting
// that silently does not exist.
//
// Reading is deliberately more forgiving than writing. A stored id this build
// no longer renders simply matches no row and is ignored, so retiring a row
// cannot lock a reader out of their own settings.
func validHiddenRows(ids []string) ([]string, error) {
	rows := make([]string, 0, len(ids))
	seen := make(map[string]bool, len(ids))

	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		if !location.KnownRow(id) {
			return nil, errcode.Errorf(errcode.PreferencesRowUnknown,
				"%q is not a row of the location panel", id)
		}
		seen[id] = true
		rows = append(rows, id)
	}

	// No count check, deliberately. There was one, and it could never fire: the
	// dedup above and the KnownRow refusal below it already bound the result to
	// the number of rows there are. A check that reads as a bound and is not one
	// is worse than no check, because the next person to relax KnownRow into a
	// drop-silently policy would believe the cap still held. The real bound is
	// maxPreferencesBody, applied to the request body before any of this runs.
	return rows, nil
}

// validZoneName bounds a row label.
//
// A label is free text: it is the reader's own, it is only ever rendered by
// React, which escapes it, and this server never interprets it. The checks are
// therefore about keeping something absurd out of the store, not about safety.
func validZoneName(name string) error {
	if len([]rune(name)) > maxZoneNameLength {
		return errcode.Errorf(errcode.PreferencesZoneNameTooLong,
			"a timezone label may be at most %d characters", maxZoneNameLength)
	}

	// Control characters would let a label disturb the layout of the row it
	// sits in, and no real name contains one.
	for _, r := range name {
		if unicode.IsControl(r) {
			return errcode.Errorf(errcode.PreferencesZoneNameControlCharacters,
				"a timezone label may not contain control characters")
		}
	}

	return nil
}

// validZoneShape reports whether a string is shaped like a timezone identifier.
//
// Split from the lookup below so validate can reject an obviously wrong blob
// without resolving anything: a request carrying dozens of entries would
// otherwise buy a tzdata lookup each before being refused on its size.
func validZoneShape(zone string) error {
	if len(zone) > maxZoneIDLength || !zoneIDRe.MatchString(zone) {
		return errcode.Errorf(errcode.PreferencesZoneIDMalformed,
			"%q is not a timezone identifier", zone)
	}

	// "Local" resolves to whatever zone the server process happens to be in.
	// That is not a place, it is not the reader's zone, and two nodes of the
	// same cluster can disagree about it.
	if zone == "Local" {
		return errcode.Errorf(errcode.PreferencesZoneIDLocal,
			`"Local" is not a timezone identifier`)
	}

	return nil
}

// validZoneID reports whether a string names a timezone this server can resolve.
func validZoneID(zone string) error {
	if err := validZoneShape(zone); err != nil {
		return err
	}

	// The tzdata this resolves against is embedded by the blank import in
	// server/main.go, so this does not depend on the host having a zoneinfo
	// database.
	if _, err := time.LoadLocation(zone); err != nil {
		return errcode.Errorf(errcode.PreferencesZoneIDUnknown,
			"unknown timezone %q", zone)
	}

	return nil
}

// preferenceStore reads and writes one preferences blob per user.
type preferenceStore interface {
	Get(userID string) (UserPreferences, error)
	Set(userID string, prefs UserPreferences) error
	Delete(userID string) error
}

// preferenceKey namespaces the blob by user. Mattermost caps a KV key at 50
// characters and a user ID is 26, so this always fits.
func preferenceKey(userID string) string { return "prefs-" + userID }

// kvPreferenceStore is the durable store, backed by the plugin KV store.
type kvPreferenceStore struct {
	api plugin.API
}

var _ preferenceStore = (*kvPreferenceStore)(nil)

// Get returns the stored blob, or the zero value when the reader has never
// saved one. The zero value means "all defaults", so a missing key is not an
// error and callers do not have to distinguish the two.
func (s *kvPreferenceStore) Get(userID string) (UserPreferences, error) {
	data, appErr := s.api.KVGet(preferenceKey(userID))
	if appErr != nil {
		return UserPreferences{}, errors.Wrap(appErr, "failed to read preferences")
	}
	if len(data) == 0 {
		return UserPreferences{}, nil
	}

	var prefs UserPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		// Defaults rather than an error, on purpose. These are view settings:
		// a blob this build can no longer read must not be able to take the
		// panel down with it, and the reader's next save replaces it.
		s.api.LogWarn("Discarding an unreadable preferences blob",
			"error_code", errcode.PreferencesBlobUnreadable, "user_id", userID, "error", err.Error())
		return UserPreferences{}, nil
	}

	return prefs, nil
}

func (s *kvPreferenceStore) Set(userID string, prefs UserPreferences) error {
	// Expected to stay uncovered: UserPreferences is plain structs, slices and
	// ints, so there is no value it can hold that Marshal refuses.
	data, err := json.Marshal(prefs)
	if err != nil {
		return errors.Wrap(err, "failed to encode preferences")
	}

	if appErr := s.api.KVSet(preferenceKey(userID), data); appErr != nil {
		return errors.Wrap(appErr, "failed to save preferences")
	}

	return nil
}

// Delete removes the blob entirely, which is how "Restore defaults" works.
// Deleting a key that is not there is not an error.
func (s *kvPreferenceStore) Delete(userID string) error {
	if appErr := s.api.KVDelete(preferenceKey(userID)); appErr != nil {
		return errors.Wrap(appErr, "failed to clear preferences")
	}

	return nil
}
