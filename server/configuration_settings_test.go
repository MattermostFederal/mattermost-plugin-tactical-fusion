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

// Every switch defaults on. The zero value of the Go field is false, so a
// missing default here would leave the plugin inert on a fresh install and look
// like a bug in decoration rather than in the manifest.
func TestEverySwitchDefaultsOn(t *testing.T) {
	for _, setting := range loadSettings(t).SettingsSchema.Settings {
		t.Run(setting.Key, func(t *testing.T) {
			if setting.Type != "bool" {
				return
			}
			if setting.Default != true {
				t.Fatalf("%q defaults to %v, want true", setting.Key, setting.Default)
			}
			if setting.HelpText == "" {
				t.Fatalf("%q has no help text, so an admin has to guess what it does", setting.Key)
			}
		})
	}
}
