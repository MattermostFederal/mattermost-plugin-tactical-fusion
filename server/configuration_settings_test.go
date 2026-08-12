package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// settingsSchema is the slice of plugin.json this plugin's admin console is
// built from.
type settingsSchema struct {
	SettingsSchema struct {
		// Header is markdown shown above the settings. It is where the built-in
		// documentation is advertised to admins; see help_docs_test.go.
		Header   string `json:"header"`
		Settings []struct {
			Key         string `json:"key"`
			DisplayName string `json:"display_name"`
			Type        string `json:"type"`
			Default     any    `json:"default"`
			HelpText    string `json:"help_text"`
		} `json:"settings"`
	} `json:"settings_schema"`
}

func loadSettings(t *testing.T) settingsSchema {
	t.Helper()

	path := filepath.Join("..", "plugin.json")
	raw, err := os.ReadFile(path) // #nosec G304 -- fixed, repo-relative manifest path
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	var schema settingsSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("plugin.json is not valid JSON: %v", err)
	}
	if len(schema.SettingsSchema.Settings) == 0 {
		t.Fatal("plugin.json declares no settings")
	}

	return schema
}

// Mattermost binds a setting to a struct field by name. A key with no field
// silently never applies: the admin toggles it, the plugin never sees it, and
// nothing anywhere reports a problem.
func TestEverySettingBindsToAConfigurationField(t *testing.T) {
	fields := map[string]reflect.StructField{}
	for field := range reflect.TypeFor[configuration]().Fields() {
		fields[strings.ToLower(field.Name)] = field
	}

	for _, setting := range loadSettings(t).SettingsSchema.Settings {
		t.Run(setting.Key, func(t *testing.T) {
			field, ok := fields[strings.ToLower(setting.Key)]
			if !ok {
				t.Fatalf("plugin.json declares %q but configuration has no such field", setting.Key)
			}
			if setting.Type == "bool" && field.Type.Kind() != reflect.Bool {
				t.Fatalf("%q is a bool in plugin.json but %s in configuration", setting.Key, field.Type)
			}
		})
	}
}

// The other direction: a field with no setting is a switch nobody can reach.
func TestEveryConfigurationFieldHasASetting(t *testing.T) {
	declared := map[string]bool{}
	for _, setting := range loadSettings(t).SettingsSchema.Settings {
		declared[strings.ToLower(setting.Key)] = true
	}

	for field := range reflect.TypeFor[configuration]().Fields() {
		if !declared[strings.ToLower(field.Name)] {
			t.Fatalf("configuration has %q but plugin.json declares no setting for it", field.Name)
		}
	}
}

// defaultsOff is every switch that ships disabled, by name.
//
// A list rather than a rule, because there is no rule: each entry is a judgment
// that the format costs more than it gives on a workspace that has not asked
// for it, and the only way to be sure one is deliberate is to have written it
// down here.
var defaultsOff = map[string]string{
	"EnableLocationUTM": "the band letter is ambiguous, so a wrong reading is a " +
		"confidently misplaced position rather than a missed decoration",
}

// Every switch declares a default, and it is the one intended.
//
// The zero value of the Go field is false, so a MISSING default leaves the
// plugin inert on a fresh install and looks like a bug in decoration rather
// than in the manifest. That is the original reason this test exists, and it
// still is: nil and false are both falsy at runtime and only this can tell them
// apart.
//
// Pinned in both directions since UTM started shipping off. A default that
// flipped either way would change what a fresh install decorates, silently, and
// for UTM specifically it would change it from "declines an ambiguous token" to
// "resolves it and may be 90 degrees of latitude out".
func TestEverySwitchDeclaresTheIntendedDefault(t *testing.T) {
	seen := map[string]bool{}

	for _, setting := range loadSettings(t).SettingsSchema.Settings {
		t.Run(setting.Key, func(t *testing.T) {
			if setting.Type != "bool" {
				return
			}

			if setting.Default == nil {
				t.Fatalf("%q declares no default, so a fresh install reads it as false", setting.Key)
			}
			if setting.HelpText == "" {
				t.Fatalf("%q has no help text, so an admin has to guess what it does", setting.Key)
			}

			why, off := defaultsOff[setting.Key]
			seen[setting.Key] = true

			if off {
				if setting.Default != false {
					t.Fatalf("%q defaults to %v, want false: %s", setting.Key, setting.Default, why)
				}
				return
			}

			if setting.Default != true {
				t.Fatalf("%q defaults to %v, want true; add it to defaultsOff with a reason "+
					"if shipping it disabled is deliberate", setting.Key, setting.Default)
			}
		})
	}

	// A name in the list that no longer matches a setting would silently stop
	// pinning anything, which is how a default comes back on unnoticed.
	for key := range defaultsOff {
		if !seen[key] {
			t.Errorf("defaultsOff names %q, which is not a switch in plugin.json", key)
		}
	}
}

// The one switch that ships off has to say so where an admin will see it.
//
// An off-by-default format is indistinguishable from a broken one: an admin
// pastes a UTM position, nothing happens, and the setting they would check is
// the one already sitting at its default. The help text is the only thing
// standing between that and a bug report.
func TestASwitchThatShipsOffSaysWhyInItsHelpText(t *testing.T) {
	for _, setting := range loadSettings(t).SettingsSchema.Settings {
		if _, off := defaultsOff[setting.Key]; !off {
			continue
		}

		t.Run(setting.Key, func(t *testing.T) {
			// The phrase, not the word. Grepping for "default" alone was
			// satisfied by help text reading "Enabled by default", which is the
			// exact inverse of what this test is for.
			if !saysItShipsOff(setting.HelpText) {
				t.Errorf("%q ships disabled but its help text never says so:\n%s",
					setting.Key, setting.HelpText)
			}

			// And the display name, since that is what an admin scanning the
			// page reads before any help text.
			if !saysItShipsOff(setting.DisplayName) {
				t.Errorf("%q ships disabled but its display name does not say so: %q",
					setting.Key, setting.DisplayName)
			}
		})
	}
}

// saysItShipsOff reports whether a string tells an admin the setting is off
// until they turn it on.
func saysItShipsOff(text string) bool {
	lower := strings.ToLower(text)

	for _, phrase := range []string{"off by default", "disabled by default"} {
		if strings.Contains(lower, phrase) {
			return true
		}
	}

	return false
}
