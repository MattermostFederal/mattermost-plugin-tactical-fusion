#!/usr/bin/env bash
set -uo pipefail

DIST="${1:-dist}"
ALLOWLIST="build/virus-allowlist.txt"

if [ ! -d "$DIST" ] || [ ! -r "$DIST" ]; then
    echo "error: $DIST is not a readable directory, so there is nothing to scan" >&2
    exit 1
fi

work=$(mktemp -d)
report=$(mktemp)
errors=$(mktemp)
listing=$(mktemp)
trap 'rm -rf "$work" "$report" "$errors" "$listing"' EXIT

for archive in "$DIST"/*.tar.gz; do
    [ -e "$archive" ] || continue
    dest="$work/$(basename "$archive" .tar.gz)"
    mkdir -p "$dest"
    if ! tar -xzf "$archive" -C "$dest"; then
        echo "error: $archive could not be unpacked for scanning" >&2
        exit 1
    fi
done

if ! find "$DIST" -mindepth 1 -maxdepth 1 ! -name '*.tar.gz' -print0 > "$listing"; then
    echo "error: $DIST could not be listed, so its files cannot all be scanned" >&2
    exit 1
fi
targets=("$work")
while IFS= read -r -d '' entry; do
    targets+=("$entry")
done < "$listing"

clamscan --recursive --infected --alert-broken "${targets[@]}" > "$report" 2> "$errors"
status=$?
cat "$report"
cat "$errors" >&2

reported_errors=$(awk -F': *' '/^Total errors:/ { print $2 }' "$report")
if [ "$status" -ne 0 ] && [ "$status" -ne 1 ]; then
    echo "error: clamscan could not complete the scan (exit $status)" >&2
    exit 1
fi
if [ "${reported_errors:-0}" -ne 0 ] || grep -q 'ERROR' "$errors"; then
    echo "error: clamscan reported errors, so a finding it allowed cannot stand for a clean scan" >&2
    exit 1
fi

allowed() {
    local path="$1" signature="$2" glob listed
    while read -r glob listed _; do
        case "$glob" in ''|\#*) continue ;; esac
        # shellcheck disable=SC2053
        if [[ "$path" == $glob ]] && [ "$signature" = "$listed" ]; then
            return 0
        fi
    done < "$ALLOWLIST"
    return 1
}

unexpected=0
while IFS= read -r line; do
    case "$line" in *" FOUND") ;; *) continue ;; esac
    path="${line%%: *}"
    signature="${line#*: }"
    signature="${signature% FOUND}"
    shown="${path#"$work"/}"
    if allowed "$path" "$signature"; then
        echo "allowed by $ALLOWLIST: $shown $signature"
    else
        echo "error: $shown matches $signature, which $ALLOWLIST does not allow" >&2
        unexpected=1
    fi
done < "$report"

if [ "$unexpected" -ne 0 ]; then
    exit 1
fi
echo "Virus scan passed."
