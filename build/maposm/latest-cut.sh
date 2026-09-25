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

MAX_LOOKBACK_DAYS="${MAX_LOOKBACK_DAYS:-14}"

previous_day() {
    if date -u -d "20${1} -1 day" +%y%m%d >/dev/null 2>&1; then
        date -u -d "20${1} -1 day" +%y%m%d
    else
        date -u -j -v-1d -f %Y%m%d "20${1}" +%y%m%d
    fi
}

status_of() {
    curl -sSI -L --retry 3 -o /dev/null -w '%{http_code}' "$1" || echo "000"
}

cut="$oldest"
for _ in $(seq 0 "$MAX_LOOKBACK_DAYS"); do
    missing=""
    for extract in $extracts; do
        code=$(status_of "${BASE}/${extract}-${cut}.osm.pbf")
        case "$code" in
            200) ;;
            404) missing="$extract"; break ;;
            *)
                echo "error: asking Geofabrik for ${extract}-${cut} answered HTTP ${code}" >&2
                exit 1
                ;;
        esac
    done
    if [ -z "$missing" ]; then
        echo "$cut"
        exit 0
    fi
    echo "  ${missing} has no extract cut on ${cut}; trying the day before" >&2
    cut=$(previous_day "$cut")
done

echo "error: no cut in the ${MAX_LOOKBACK_DAYS} days before ${oldest} is published for every extract in '${PROFILE}'" >&2
exit 1
