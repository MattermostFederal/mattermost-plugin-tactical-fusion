#!/usr/bin/env bash
set -uo pipefail

DIST="${1:-dist}"
ALLOWLIST="build/virus-allowlist.txt"

work=$(mktemp -d)
report=$(mktemp)
trap 'rm -rf "$work" "$report"' EXIT

for archive in "$DIST"/*.tar.gz; do
    [ -e "$archive" ] || continue
    dest="$work/$(basename "$archive" .tar.gz)"
    mkdir -p "$dest"
    if ! tar -xzf "$archive" -C "$dest"; then
        echo "error: $archive could not be unpacked for scanning" >&2
        exit 1
    fi
done

clamscan --recursive --infected --alert-broken --exclude='\.tar\.gz$' "$DIST" "$work" > "$report"
status=$?
cat "$report"

if [ "$status" -ne 0 ] && [ "$status" -ne 1 ]; then
    echo "error: clamscan could not complete the scan (exit $status)" >&2
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
