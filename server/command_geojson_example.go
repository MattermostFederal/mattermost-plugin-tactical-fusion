package main

import (
	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

// geoJSONExample is a small overlay of the kind a GIS export produces: a point,
// a route and an area, each named and each carrying the sort of properties a
// real document attaches, each styled the way simplestyle-spec says: the area
// is red at a quarter fill, the route and its supply point blue.
//
// Styled rather than left to the theme because the styling is a feature and an
// example that stated none demonstrated only that this build ignores it. The
// area is the one worth looking at: fill-opacity 0.25 is a quarter, not a
// quarter of the 0.16 an unstyled shape gets.
//
// The route bends and the area is a twenty point blob, because a three point
// line and an axis-aligned rectangle are what a hand-typed document looks like
// and not what a GIS export does. They also drew a shape whose rendering proved
// nothing: a rectangle is the one polygon that looks correct even if the
// renderer swapped its axis order or dropped every vertex but the corners of
// its bounding box. The area contains the point feature, so a reader can see
// the two relate rather than sitting side by side.
//
// Still deliberately small. The examples command measures every message against
// safePostRunes and refuses the whole run rather than posting some of it, so an
// example sized to show off the caps would cost the reader every other example
// on a server with a smaller limit.
const geoJSONExample = `{
  "type": "FeatureCollection",
  "name": "Pearl Harbor resupply",
  "description": "Supply point, route in and operating area for the exercise.",
  "features": [
    {
      "type": "Feature",
      "geometry": {"type": "Point", "coordinates": [-157.9483, 21.3353]},
      "properties": {
        "name": "Forward supply point",
        "description": "Class I and III issue point, open 0600 to 2200 local.",
        "status": "active", "capacity": 240, "fuel": "JP-8",
        "operator": "1st Battalion S4", "opened": "2026-09-22",
        "marker-color": "#0000ff", "marker-size": "large"
      }
    },
    {
      "type": "Feature",
      "geometry": {"type": "LineString", "coordinates": [
        [-157.9513, 21.3315], [-157.9487, 21.3380], [-157.9444, 21.3414],
        [-157.9337, 21.3435], [-157.9243, 21.3478], [-157.9213, 21.3550],
        [-157.9176, 21.3624], [-157.9074, 21.3670], [-157.8985, 21.3742],
        [-157.8890, 21.3798], [-157.8801, 21.3871], [-157.8715, 21.3958],
        [-157.8612, 21.4025]
      ]},
      "properties": {
        "name": "Primary route",
        "description": "Convoy route from the pier to the supply point, two lanes throughout.",
        "surface": "paved", "lanes": 2, "speed_limit": "40 km/h",
        "checkpoints": 3, "condition": "green",
        "stroke": "#00a651", "stroke-width": 3, "stroke-opacity": 0.9
      }
    },
    {
      "type": "Feature",
      "geometry": {"type": "MultiPolygon", "coordinates": [
        [
          [
            [-157.9065, 21.3455], [-157.9085, 21.3534], [-157.9148, 21.3588],
            [-157.9221, 21.3613], [-157.9279, 21.3644], [-157.9345, 21.3621],
            [-157.9404, 21.3624], [-157.9525, 21.3686], [-157.9597, 21.3626],
            [-157.9548, 21.3517], [-157.9527, 21.3455], [-157.9524, 21.3401],
            [-157.9522, 21.3335], [-157.9493, 21.3266], [-157.9422, 21.3235],
            [-157.9345, 21.3191], [-157.9261, 21.3215], [-157.9254, 21.3338],
            [-157.9224, 21.3373], [-157.9109, 21.3384], [-157.9065, 21.3455]
          ],
          [
            [-157.9412, 21.3352], [-157.9446, 21.3379], [-157.9451, 21.3418],
            [-157.9423, 21.3449], [-157.9384, 21.3461], [-157.9349, 21.3440],
            [-157.9338, 21.3402], [-157.9357, 21.3368], [-157.9412, 21.3352]
          ]
        ]
      ]},
      "properties": {
        "name": "Operating area",
        "description": "Restricted to exercise traffic for the duration. The inner block is the hospital grounds and stays open.",
        "restriction": "exercise traffic only", "effective": "2026-09-22T06:00Z",
        "expires": "2026-09-30T22:00Z", "poc": "Exercise control, S3",
        "stroke": "#ff0000", "stroke-width": 3, "stroke-opacity": 0.8,
        "fill": "#ff0000", "fill-opacity": 0.25
      }
    }
  ]
}`

const geoJSONExampleLead = "A GeoJSON document, the format every GIS export, mapping API and " +
	"OpenStreetMap extract speaks. The card says what the document holds and names each " +
	"feature, with the properties it carries. By default only a fence labeled `geojson` is " +
	"read, and a `json` one is left alone, because rendering an ordinary JSON payload this " +
	"way would cost that post its search matches permanently. An admin can widen this with " +
	"the \"Also read GeoJSON that does not say it is GeoJSON\" setting."

// geoJSONExampleMessages is what this command will post, for the size gate that
// runs before anything is written.
func (p *Plugin) geoJSONExampleMessages() []string {
	if !p.geoJSONEnabled() {
		return nil
	}

	return []string{geoJSONExampleMessage()}
}

func geoJSONExampleMessage() string {
	return geoJSONExampleLead + "\n\n```geojson\n" + geoJSONExample + "\n```\n"
}

func (p *Plugin) geoJSONExampleCount() int {
	if !p.geoJSONEnabled() {
		return 0
	}
	return 1
}

// postGeoJSONExamples returns how many of the examples could not be posted.
func (p *Plugin) postGeoJSONExamples(args *model.CommandArgs) int {
	if !p.geoJSONEnabled() {
		return 0
	}

	post := &model.Post{
		UserId:    args.UserId,
		ChannelId: args.ChannelId,
		RootId:    args.RootId,
		Message:   geoJSONExampleMessage(),
	}

	// Stamped here rather than left to the hook, for the reason the Cursor on
	// Target examples are: CreatePost from a plugin does not run
	// MessageWillBePosted, so an unstamped example would post as raw JSON.
	if card, stamped := p.geoJSONStamp(post); stamped {
		post = card
	}

	if _, appErr := p.API.CreatePost(post); appErr != nil {
		p.API.LogError("tactical-fusion: could not post a GeoJSON example",
			"error_code", errcode.CommandExamplesPostFailed, "error", appErr.Error())
		return 1
	}

	return 0
}
