package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// setting is one switch as plugin.json declares it.
type setting struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	Default     any    `json:"default"`
	HelpText    string `json:"help_text"`
}

// settingsSection is one group in the admin console.
type settingsSection struct {
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	Subtitle string    `json:"subtitle"`
	Settings []setting `json:"settings"`
}

// settingsSchema is the slice of plugin.json this plugin's admin console is
// built from.
//
// Both Settings and Sections are parsed, and every switch lives in a section:
// the flat list is kept in the struct only so the test below can insist it is
// empty. A key left at the top level renders outside every group heading, above
// the first one, which reads as a switch belonging to no feature.
type settingsSchema struct {
	SettingsSchema struct {
		// Header is markdown shown above the settings. It is where the built-in
		// documentation is advertised to admins; see help_docs_test.go.
		Header   string            `json:"header"`
		Settings []setting         `json:"settings"`
		Sections []settingsSection `json:"sections"`
	} `json:"settings_schema"`
}

// allSettings is every switch, whichever section declares it.
//
// The tests below are about the switches rather than the arrangement, so they
// take this and the grouping is asserted separately. Without it, moving the
// settings into sections would have left every one of them iterating an empty
// slice and passing while checking nothing.
func (s settingsSchema) allSettings() []setting {
	var all []setting
	all = append(all, s.SettingsSchema.Settings...)
	for _, section := range s.SettingsSchema.Sections {
		all = append(all, section.Settings...)
	}

	return all
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
	if len(schema.allSettings()) == 0 {
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

	for _, setting := range loadSettings(t).allSettings() {
		t.Run(setting.Key, func(t *testing.T) {
			// A "custom" setting stores nothing. Mattermost renders a component
			// the webapp registered under the key instead, so there is no value
			// to bind and a configuration field for one would be a field
			// nothing ever writes. Every other type is a value and is held to
			// the rule above.
			if setting.Type == "custom" {
				t.Skipf("%q renders a component rather than storing a value", setting.Key)
			}

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
	for _, setting := range loadSettings(t).allSettings() {
		declared[strings.ToLower(setting.Key)] = true
	}

	for field := range reflect.TypeFor[configuration]().Fields() {
		if !declared[strings.ToLower(field.Name)] {
			t.Fatalf("configuration has %q but plugin.json declares no setting for it", field.Name)
		}
	}
}

// Every switch belongs to a section, and no section is anonymous.
//
// Mattermost renders settings_schema.settings above the first section heading,
// so a key left there is a switch that appears to belong to no feature. That is
// exactly the arrangement sections replaced, and it is the state the manifest
// drifts back into one setting at a time if nothing says otherwise.
//
// A section with no key is refused by the server outright (PluginSettingsSection
// .IsValid), which makes the whole plugin fail to load rather than merely look
// wrong. A section with no title renders as an unlabelled group, which is worse
// than no grouping at all.
func TestEverySettingBelongsToANamedSection(t *testing.T) {
	schema := loadSettings(t).SettingsSchema

	for _, setting := range schema.Settings {
		t.Errorf("%q sits outside every section, so it renders above the first heading "+
			"with nothing saying what it governs", setting.Key)
	}

	if len(schema.Sections) == 0 {
		t.Fatal("plugin.json declares no sections")
	}

	keys := map[string]bool{}
	for _, section := range schema.Sections {
		if section.Key == "" {
			t.Error("a section declares no key, which Mattermost refuses at load time")
			continue
		}
		if keys[section.Key] {
			t.Errorf("two sections share the key %q", section.Key)
		}
		keys[section.Key] = true

		if section.Title == "" {
			t.Errorf("section %q declares no title, so its switches group under nothing", section.Key)
		}
		if len(section.Settings) == 0 {
			t.Errorf("section %q declares no settings", section.Key)
		}
	}

	// And no setting key appears twice, in one section or across two.
	//
	// Nothing else catches it: allSettings appends, the binding tests collapse
	// into maps, and Go silently disambiguates duplicate t.Run names. A repeated
	// key renders two switches bound to one configuration field, so flipping one
	// appears not to stick, which is the same admin-facing silence the binding
	// tests exist to prevent.
	seen := map[string]string{}
	for _, section := range schema.Sections {
		for _, setting := range section.Settings {
			if where, dup := seen[setting.Key]; dup {
				t.Errorf("setting %q is declared in both the %q and %q sections",
					setting.Key, where, section.Key)
			}
			seen[setting.Key] = section.Key
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

	"EnableGeoJSONUnlabeled": "stamping a post is permanent and costs it its search " +
		"matches, and ordinary JSON is pasted into chat constantly, so reading a json " +
		"fence, a .json attachment or an unfenced object has to be asked for rather " +
		"than arrived at",
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

	for _, setting := range loadSettings(t).allSettings() {
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
	for _, setting := range loadSettings(t).allSettings() {
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

// Both generated manifests embed plugin.json verbatim inside a literal a
// backtick terminates: server/manifest.go uses a Go raw string and
// webapp/src/manifest.ts a JavaScript template literal. So a backtick anywhere
// in plugin.json breaks BOTH builds, and it breaks them hundreds of lines from
// the cause, in files nobody edits and git does not track.
//
// Markdown fences in help text are the way this happens, since a fenced block
// is exactly what an admin reading about Cursor on Target wants described.
func TestPluginManifestCarriesNoBacktick(t *testing.T) {
	raw, err := os.ReadFile("../plugin.json")
	if err != nil {
		t.Fatalf("could not read plugin.json: %v", err)
	}

	if before, _, found := bytes.Cut(raw, []byte("`")); found {
		t.Errorf("plugin.json carries a backtick on line %d; both generated manifests embed "+
			"this file inside a literal it terminates, so it breaks the Go and the webapp "+
			"build at once. Describe a fenced block in words instead",
			1+bytes.Count(before, []byte("\n")))
	}
}
