package main

import (
	// Embed the IANA timezone database in the plugin binary. time.LoadLocation
	// otherwise reads the host's copy, which minimal container images often do
	// not ship. Without this, UTC keeps working while every other zone fails at
	// runtime, so the breakage hides from a casual smoke test.
	_ "time/tzdata"

	"github.com/mattermost/mattermost/server/public/plugin"
)

func main() {
	plugin.ClientMain(&Plugin{})
}
