#!/usr/bin/env bash
set -euo pipefail

COARSE="${COARSE:-build/mapdata/source}"
FINE="${FINE:-build/maptiles/source}"
WORK="${WORK:-build/maptiles/work}"
OUT="${OUT:-public/map/world.pmtiles}"

# Four times tippecanoe's default. Override to measure what detail it is
# costing: SIMPLIFICATION=2 make map-tiles
SIMPLIFICATION="${SIMPLIFICATION:-4}"

# The archive's ceiling, written once. Every run that reaches it reads this, so
# the depth cannot move for one layer and miss another; DATA_MAX_ZOOM in
# webapp/src/decorators/location/map/span.ts must move with it, and
# TestArchiveDepthMatchesTheData is what holds the two together.
#
# NOT the camera's ceiling. MAX_ZOOM in that file is 17 and deliberately runs
# past the data, so a reader can see how small a fine cell is; pairing this with
# MAX_ZOOM would collapse the two back into one and take that with it.
MAXZ="${MAXZ:-9}"

mkdir -p "$WORK" "$(dirname "$OUT")"

# A tile with no ceiling is the failure this plugin is built to avoid: the base
# and label tiers need one so every country label survives z0, but a z8 tile over
# the Ruhr carrying roads, rail, urban and admin-1 has no natural bound at all.
# MAX_TILE_BYTES scopes a limit to the runs that need it.
tippecanoe_common=(
    --no-feature-limit
    --no-tile-stats
    --drop-rate=1
    --detect-shared-borders
    --quiet
)

parts=()

# One tippecanoe run per zoom band, joined at the end. tippecanoe takes a zoom
# range per invocation and not per layer, so a layer that should only appear
# once the reader is close needs a run of its own.
run() {
    local name="$1" minz="$2" maxz="$3"
    shift 3

    local -a bound=(--no-tile-size-limit)
    if [ -n "${MAX_TILE_BYTES:-}" ]; then
        bound=(--maximum-tile-bytes="$MAX_TILE_BYTES" --drop-densest-as-needed)
    fi

    tippecanoe "${tippecanoe_common[@]}" "${bound[@]}" \
        --simplification="$SIMPLIFICATION" \
        --output="${WORK}/part_${name}.pmtiles" --force \
        --minimum-zoom="$minz" --maximum-zoom="$maxz" \
        "$@"

    parts+=("${WORK}/part_${name}.pmtiles")
    echo "  ${name}  z${minz}-${maxz}  $(wc -c < "${WORK}/part_${name}.pmtiles") bytes"
}

# The coastline trio, at the scale that suits each zoom band.
outline() {
    local dir="$1" scale="$2" minz="$3" maxz="$4"

    for layer in land lakes admin_0_boundary_lines_land; do
        python3 build/maptiles/strip.py \
            "${dir}/ne_${scale}_${layer}.geojson" \
            "${WORK}/${scale}_${layer}.geojson"
    done

    run "outline_${scale}" "$minz" "$maxz" \
        --named-layer=land:"${WORK}/${scale}_land.geojson" \
        --named-layer=lakes:"${WORK}/${scale}_lakes.geojson" \
        --named-layer=boundary_lines:"${WORK}/${scale}_admin_0_boundary_lines_land.geojson"
}

strip_fine() {
    local layer="$1"
    shift
    python3 build/maptiles/strip.py \
        "${FINE}/ne_10m_${layer}.geojson" "${WORK}/10m_${layer}.geojson" "$@"
}

python3 build/maptiles/labels.py \
    "${COARSE}/ne_110m_admin_0_countries.geojson" \
    "${WORK}/country_labels.geojson"

node build/maptiles/glyphs.js

outline "$COARSE" 110m 0 2
outline "$FINE"   50m  3 4
outline "$FINE"   10m  5 6

# The same three layers again, deeper and sharper. Douglas-Peucker tolerance
# scales as 1/2^z, so dropping to full detail costs 81% more vertices at z5 and
# only 11% at z8: the zooms where the shape is visible are the zooms where
# keeping it is cheap. z0-6 stays byte-identical to before.
SIMPLIFICATION=1 run outline_10m_deep 7 "$MAXZ" \
    --named-layer=land:"${WORK}/10m_land.geojson" \
    --named-layer=lakes:"${WORK}/10m_lakes.geojson" \
    --named-layer=boundary_lines:"${WORK}/10m_admin_0_boundary_lines_land.geojson"

# Everything below is 10m only, and each is held back to the zoom where it
# starts being worth its bytes rather than drawn at every scale.
strip_fine rivers_lake_centerlines
strip_fine admin_1_states_provinces_lines
run context 4 "$MAXZ" \
    --named-layer=rivers:"${WORK}/10m_rivers_lake_centerlines.geojson" \
    --named-layer=admin_1_lines:"${WORK}/10m_admin_1_states_provinces_lines.geojson"

strip_fine urban_areas
strip_fine roads scalerank
strip_fine railroads
MAX_TILE_BYTES=250000 run detail 5 "$MAXZ" \
    --coalesce \
    --named-layer=urban_areas:"${WORK}/10m_urban_areas.geojson" \
    --named-layer=roads:"${WORK}/10m_roads.geojson" \
    --named-layer=railroads:"${WORK}/10m_railroads.geojson"

strip_fine populated_places NAME_EN SCALERANK
run places 3 "$MAXZ" \
    --named-layer=populated_places:"${WORK}/10m_populated_places.geojson"

strip_fine airports NAME SCALERANK
strip_fine admin_1_label_points NAME SCALERANK
run sites 5 "$MAXZ" \
    --named-layer=airports:"${WORK}/10m_airports.geojson" \
    --named-layer=admin_1_labels:"${WORK}/10m_admin_1_label_points.geojson"

run labels 0 "$MAXZ" \
    --named-layer=country_labels:"${WORK}/country_labels.geojson"

rm -f "$OUT"
tile-join --output="$OUT" --no-tile-size-limit --quiet "${parts[@]}"

ls -l "$OUT"
shasum -a 256 "$OUT" 2>/dev/null || sha256sum "$OUT"
