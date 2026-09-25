#!/usr/bin/env bash
set -euo pipefail

BASE="${GEOFABRIK_BASE:-https://download.geofabrik.de}"
REGIONS="build/maposm/regions.txt"
PROFILE="${PROFILE:?set PROFILE to the regions whose common cut is wanted}"

extracts=$(grep -v '^#' "$REGIONS" |
    awk -v p="$PROFILE" 'NF {
        if (index("," $2 ",", "," p ",") == 0) next
        n = split($3, a, ","); for (i = 1; i <= n; i++) print a[i]
    }' |
    sort -u)

if [ -z "$extracts" ]; then
    echo "error: profile '${PROFILE}' selects no region in ${REGIONS}" >&2
    exit 1
fi

oldest=""
for extract in $extracts; do
    dated=$(curl -fsSLI --retry 3 -o /dev/null -w '%{url_effective}' "${BASE}/${extract}-latest.osm.pbf")
    cut=$(basename "$dated" | sed -n 's/.*-\([0-9]\{6\}\)\.osm\.pbf$/\1/p')
    if [ -z "$cut" ]; then
        echo "error: ${extract}-latest did not redirect to a dated extract: ${dated}" >&2
        exit 1
    fi
    echo "  ${extract}: ${cut}" >&2
    if [ -z "$oldest" ] || [ "$cut" -lt "$oldest" ]; then
        oldest="$cut"
    fi
done

for extract in $extracts; do
    if ! curl -fsSI --retry 3 -o /dev/null "${BASE}/${extract}-${oldest}.osm.pbf"; then
        echo "error: ${extract} has no extract cut on ${oldest}, the newest date every region shares" >&2
        exit 1
    fi
done

echo "$oldest"
