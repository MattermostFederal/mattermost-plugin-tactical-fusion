#!/usr/bin/env bash
set -euo pipefail

COARSE="${COARSE:-build/mapdata/source}"
FINE="${FINE:-build/maptiles/source}"
WORK="${WORK:-build/maptiles/work}"
OUT="${OUT:-public/map/world.pmtiles}"

mkdir -p "$WORK" "$(dirname "$OUT")"

tippecanoe_common=(
    --no-feature-limit
    --no-tile-size-limit
    --no-tile-stats
    --drop-rate=1
    --simplification=4
    --detect-shared-borders
    --quiet
)

prepare_tier() {
    local dir="$1" scale="$2"
    for layer in land lakes admin_0_boundary_lines_land; do
        python3 build/maptiles/strip.py \
            "${dir}/ne_${scale}_${layer}.geojson" \
            "${WORK}/${scale}_${layer}.geojson"
    done
}

build_tier() {
    local scale="$1" minz="$2" maxz="$3"
    tippecanoe "${tippecanoe_common[@]}" \
        --output="${WORK}/tier_${scale}.pmtiles" --force \
        --minimum-zoom="$minz" --maximum-zoom="$maxz" \
        --named-layer=land:"${WORK}/${scale}_land.geojson" \
        --named-layer=lakes:"${WORK}/${scale}_lakes.geojson" \
        --named-layer=boundary_lines:"${WORK}/${scale}_admin_0_boundary_lines_land.geojson"
    echo "  tier ${scale}  z${minz}-${maxz}  $(wc -c < "${WORK}/tier_${scale}.pmtiles") bytes"
}

node build/maptiles/glyphs.js

python3 build/maptiles/labels.py \
    "${COARSE}/ne_110m_admin_0_countries.geojson" \
    "${WORK}/country_labels.geojson"

prepare_tier "$COARSE" 110m
prepare_tier "$FINE" 50m
prepare_tier "$FINE" 10m

build_tier 110m 0 2
build_tier 50m 3 4
build_tier 10m 5 6

tippecanoe "${tippecanoe_common[@]}" \
    --output="${WORK}/tier_labels.pmtiles" --force \
    --minimum-zoom=0 --maximum-zoom=6 \
    --named-layer=country_labels:"${WORK}/country_labels.geojson"

rm -f "$OUT"
tile-join --output="$OUT" --no-tile-size-limit --quiet \
    "${WORK}/tier_110m.pmtiles" \
    "${WORK}/tier_50m.pmtiles" \
    "${WORK}/tier_10m.pmtiles" \
    "${WORK}/tier_labels.pmtiles"

ls -l "$OUT"
shasum -a 256 "$OUT" 2>/dev/null || sha256sum "$OUT"
