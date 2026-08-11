package main

import (
	"errors"
	"testing"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/dtg"
)

// OnConfigurationChange is the only reason an admin console change takes effect.
// It is called at activation and again on every save.
func TestOnConfigurationChangeLoadsTheSettings(t *testing.T) {
	p, api := newActivationPlugin(t)
	api.loadedConfig = `{
		"EnableDTG": true,
		"EnableDTGMilitary": true,
		"EnableDTGMoniker": false,
		"EnableDTGTimestamp": true
	}`

	if err := p.OnConfigurationChange(); err != nil {
		t.Fatalf("OnConfigurationChange returned an error: %v", err)
	}

	config := p.getConfiguration()
	want := configuration{
		EnableDTG:          true,
		EnableDTGMilitary:  true,
		EnableDTGMoniker:   false,
		EnableDTGTimestamp: true,
	}
	if *config != want {
		t.Fatalf("configuration = %+v, want %+v", *config, want)
	}

	// The formats the decorator will actually be asked for, which is the point
	// of loading any of this.
	formats := p.dtgFormats()
	if !formats.Military || formats.Moniker || !formats.Timestamp {
		t.Fatalf("dtgFormats() = %+v, want the loaded settings", formats)
	}
}

// A failed load must not replace a working configuration with a zero one.
// Every field is false at zero, so silently accepting the failure would turn
// decoration off across the server and look like a bug in the decorator.
func TestOnConfigurationChangeKeepsTheOldSettingsOnFailure(t *testing.T) {
	p, api := newActivationPlugin(t)

	p.setConfiguration(&configuration{EnableDTG: true, EnableDTGMilitary: true})

	api.loadErr = errors.New("configuration is unreadable")

	err := p.OnConfigurationChange()
	if err == nil {
		t.Fatal("OnConfigurationChange reported success despite the load failing")
	}
	if !errors.Is(err, api.loadErr) {
		t.Fatalf("returned %v, which does not wrap the underlying failure", err)
	}

	if config := p.getConfiguration(); !config.EnableDTG || !config.EnableDTGMilitary {
		t.Fatalf("configuration = %+v after a failed load, want the previous one left in place", *config)
	}
}

// A read before anything has been loaded must not dereference nil. It happens:
// a message can arrive between the plugin loading and its configuration
// arriving.
func TestGetConfigurationBeforeAnythingIsLoaded(t *testing.T) {
	p := &Plugin{}

	config := p.getConfiguration()
	if config == nil {
		t.Fatal("getConfiguration returned nil")
	}
	if (*config != configuration{}) {
		t.Fatalf("configuration = %+v, want the zero value", *config)
	}

	// The zero value decorates nothing, which is the deliberate choice: rather
	// decorate nothing than rewrite somebody's message on a guess.
	if formats := p.dtgFormats(); (formats != dtg.Formats{}) {
		t.Fatalf("dtgFormats() = %+v before loading, want every format off", formats)
	}
}

// setConfiguration guards against a caller handing back the live configuration
// and then mutating it, which would change behaviour for every in-flight
// message without taking the lock.
func TestSetConfigurationRejectsTheLiveConfiguration(t *testing.T) {
	p := &Plugin{}
	config := &configuration{EnableDTG: true}
	p.setConfiguration(config)

	defer func() {
		if recover() == nil {
			t.Fatal("setConfiguration accepted the configuration it already holds")
		}
	}()

	p.setConfiguration(config)
}

// A different pointer with the same contents is an ordinary replacement.
func TestSetConfigurationAcceptsAnEquivalentCopy(t *testing.T) {
	p := &Plugin{}
	p.setConfiguration(&configuration{EnableDTG: true})
	p.setConfiguration(&configuration{EnableDTG: true})

	if !p.getConfiguration().EnableDTG {
		t.Fatal("the replacement did not take effect")
	}
}

// Clone is what lets a caller read the configuration without holding a
// reference into the live one.
func TestCloneDoesNotShareStorage(t *testing.T) {
	original := &configuration{EnableDTG: true, EnableDTGMilitary: true}

	clone := original.Clone()
	if *clone != *original {
		t.Fatalf("clone = %+v, want %+v", *clone, *original)
	}

	clone.EnableDTG = false
	if !original.EnableDTG {
		t.Fatal("mutating the clone changed the original")
	}
}
