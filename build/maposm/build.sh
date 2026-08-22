#!/usr/bin/env bash
set -euo pipefail

PROFILE="${PROFILE:-pilot}"
HERE="build/maposm"
SRC="${SRC:-$HERE/source}"
WORK="${WORK:-$HERE/work}"
CACHE="${CACHE:-$HERE/cache}"
OUT="${OUT:-public/map/packages}"
REL="${REL:-$HERE/out}"
JAR="${PLANETILER_JAR:-/opt/planetiler/planetiler-openmaptiles.jar}"
JAVA_HEAP="${JAVA_HEAP:-}"

MINZ="${MINZ:-10}"
MAXZ="${MAXZ:-14}"

NAME_PATTERN='^[a-z0-9]+(-[a-z0-9]+)+$'
BBOX_PATTERN='^-?[0-9.]+,-?[0-9.]+,-?[0-9.]+,-?[0-9.]+$'
BUNDLED_PROFILE=bundled
MIN_HEAP_GB=4

KEEP_LAYERS="transportation,transportation_name,place,water,water_name,waterway,aeroway,aerodrome_label,boundary"

mkdir -p "$WORK" "$CACHE" "$OUT" "$REL"

heap_for() {
    mb=$(( $(wc -c < "$1") / 1048576 ))
    if [ "$mb" -lt 256 ]; then
        echo 4
    elif [ "$mb" -lt 1024 ]; then
        echo 6
    else
        echo 8
    fi
}

available_bytes() {
    limit=""
    if [ -r /sys/fs/cgroup/memory.max ]; then
        limit=$(cat /sys/fs/cgroup/memory.max)
    elif [ -r /sys/fs/cgroup/memory/memory.limit_in_bytes ]; then
        limit=$(cat /sys/fs/cgroup/memory/memory.limit_in_bytes)
    fi
    case "$limit" in
        ''|max|*[!0-9]*) limit=0 ;;
    esac
    if { [ "$limit" -eq 0 ] || [ "$limit" -gt 1000000000000 ]; } && [ -r /proc/meminfo ]; then
        limit=$(( $(awk '/^MemTotal:/ {print $2}' /proc/meminfo) * 1024 ))
    fi
    echo "$limit"
}

built=0

while read -r name profiles extract bbox <&3; do
    case "$name" in ''|\#*) continue ;; esac
    case ",${profiles}," in *",${PROFILE},"*) ;; *) continue ;; esac

    if ! echo "$name" | grep -Eq "$NAME_PATTERN"; then
        echo "error: '${name}' is not a package name. Expected <command>-<area>," >&2
        echo "  lower case, alphanumerics and single hyphens, because it reaches a URL." >&2
        exit 1
    fi

    if ! echo "$bbox" | grep -Eq "$BBOX_PATTERN"; then
        echo "error: ${name} bbox '${bbox}' is not west,south,east,north." >&2
        echo "  The extract column takes commas but no spaces, and nothing may follow" >&2
        echo "  the bbox: it is the last field and swallows the rest of the line." >&2
        exit 1
    fi

    case ",${profiles}," in
        *",${BUNDLED_PROFILE},"*) dest="$OUT" ;;
        *)                        dest="$REL" ;;
    esac

    IFS=, read -r -a wanted <<< "$extract"

    pbfs=()
    for path in "${wanted[@]}"; do
        leaf="${path##*/}"
        file=$(awk -v r="$leaf" '!/^#/ && NF >= 2 {
            b = $2
            sub(/-[0-9][0-9][0-9][0-9][0-9][0-9]\.osm\.pbf$/, "", b)
            if (b == r) { print $2; exit }
        }' "$HERE/sources.lock")
        if [ -z "$file" ]; then
            echo "error: '${path}' is not pinned in ${HERE}/sources.lock; run 'make osm-sources'" >&2
            exit 1
        fi
        if [ ! -f "${SRC}/${file}" ]; then
            echo "error: ${file} is pinned but not in ${SRC}; run 'make osm-sources'" >&2
            exit 1
        fi
        pbfs+=("${SRC}/${file}")
    done

    if [ "${#pbfs[@]}" -eq 1 ]; then
        pbf="${pbfs[0]}"
    else
        pbf="${WORK}/${name}.osm.pbf"
        stamp="${pbf}.inputs"
        inputs=$(printf '%s\n' "${pbfs[@]##*/}" | sort)

        cut=$(printf '%s\n' "$inputs" | sed 's/.*-\([0-9]\{6\}\)\.osm\.pbf$/\1/' | sort -u)
        if [ "$(printf '%s\n' "$cut" | wc -l)" -ne 1 ]; then
            if [ "${ALLOW_MIXED_DATES:-}" = "1" ]; then
                echo "  WARNING: ${name} merges extracts cut on different days: $(echo $cut)" >&2
                echo "  A border object edited between those cuts may appear twice." >&2
            else
                echo "error: ${name} merges extracts cut on different days:" >&2
                printf '  %s\n' $cut >&2
                echo "  osmium merge is only correct across files from one planet snapshot;" >&2
                echo "  across snapshots a border object appears twice with two versions." >&2
                echo "  Geofabrik publishes regions through the day, so a set can straddle a" >&2
                echo "  rollover. Re-pin once upstream has caught up:" >&2
                echo "    UPDATE_LOCK=1 make osm-sources PROFILE=${PROFILE}" >&2
                echo "  or accept the risk with ALLOW_MIXED_DATES=1." >&2
                exit 1
            fi
        fi

        if [ ! -f "$pbf" ] || [ "$(cat "$stamp" 2>/dev/null || true)" != "$inputs" ]; then
            rm -f "$pbf" "$stamp"
            echo "  merging ${#pbfs[@]} extracts into $(basename "$pbf")"
            osmium merge --overwrite --output="$pbf" "${pbfs[@]}"
            printf '%s\n' "$inputs" > "$stamp"
        fi
    fi

    have=$(available_bytes)
    room=$(( have / 1073741824 - 1 ))

    if [ -n "$JAVA_HEAP" ]; then
        heap="$JAVA_HEAP"
        if [ "$have" -gt 0 ] && [ "$room" -lt "${heap%g}" ]; then
            echo "error: JAVA_HEAP=${heap} was asked for and this container has $(( have / 1048576 )) MiB." >&2
            echo "  Raise Docker's memory (Docker Desktop: Settings > Resources > Memory)," >&2
            echo "  or lower JAVA_HEAP." >&2
            exit 1
        fi
    else
        want=$(heap_for "$pbf")
        heap="${want}g"
        if [ "$have" -gt 0 ] && [ "$room" -lt "$want" ]; then
            if [ "$room" -lt "$MIN_HEAP_GB" ]; then
                echo "error: ${name} needs at least $(( MIN_HEAP_GB + 1 )) GiB and this container has $(( have / 1048576 )) MiB." >&2
                echo "  Raise Docker's memory (Docker Desktop: Settings > Resources > Memory)." >&2
                exit 1
            fi
            echo "  ${name} prefers -Xmx${want}g; this container has $(( have / 1048576 )) MiB, using -Xmx${room}g"
            heap="${room}g"
        fi
    fi

    echo "  ${name}  building with -Xmx${heap}"

    java "-Xmx${heap}" -jar "$JAR" \
        --osm_path="$pbf" \
        --output="${WORK}/${name}.raw.pmtiles" \
        --bounds="$bbox" \
        --minzoom="$MINZ" \
        --maxzoom="$MAXZ" \
        --only_layers="$KEEP_LAYERS" \
        --output_layerstats \
        --download \
        --download_dir="$CACHE" \
        --tmpdir="${WORK}/tmp" \
        --tile_weights="${CACHE}/tile_weights.tsv.gz" \
        --force

    rm -f "${dest}/${name}.pmtiles"
    tile-join --output="${dest}/${name}.pmtiles" --force --no-tile-size-limit --quiet \
        -j "$(cat "$HERE/filter.json")" \
        "${WORK}/${name}.raw.pmtiles"

    raw=$(wc -c < "${WORK}/${name}.raw.pmtiles")
    final=$(wc -c < "${dest}/${name}.pmtiles")
    echo "  ${name}  z${MINZ}-${MAXZ}  -Xmx${heap}  ${raw} -> ${final} bytes  -> ${dest}"
    shasum -a 256 "${dest}/${name}.pmtiles" 2>/dev/null || sha256sum "${dest}/${name}.pmtiles"

    built=$((built + 1))
done 3< "$HERE/regions.txt"

if [ "$built" -eq 0 ]; then
    echo "error: profile '${PROFILE}' selects no region in ${HERE}/regions.txt" >&2
    exit 1
fi

ls -l "$OUT" "$REL"
