#!/usr/bin/env bash
set -euo pipefail

TAG="${1:?usage: previous-release.sh <release tag>}"
REGIONS="build/maposm/regions.txt"
PROFILE="${MAP_RELEASE_PROFILE:-release}"
RECIPE="build/maposm/regions.txt build/maposm/build.sh build/maposm/filter.json build/maposm/Dockerfile"
TOOLCHAIN_PINS='^(TIPPECANOE_VERSION|PLANETILER_OMT_VERSION|PLANETILER_OMT_SHA256) '

rebuild() {
    echo "rebuilding the map packages: $*" >&2
    exit 0
}

toolchain_pins() {
    git show "$1:Makefile" | grep -E "$TOOLCHAIN_PINS"
}

current="${TAG#v}"
major="${current%%.*}"
releases=$(gh api --paginate 'repos/{owner}/{repo}/releases' \
    --jq '.[] | select(.draft | not) | select(.prerelease | not) | .tag_name' |
    sed -n 's/^v\([0-9]*\.[0-9]*\.[0-9]*\)$/\1/p')
earlier=$(printf '%s\n' "$releases" "$current" | sort -uV | awk -v c="$current" '$0 == c { exit } NF { print }' | sort -rV)

[ -n "$earlier" ] || rebuild "no published release precedes ${TAG}"

wanted=$(printf '%s\n' PACKAGES.sha256 MAP-SOURCES.lock
    grep -v '^#' "$REGIONS" | awk -v p="$PROFILE" 'NF && index("," $2 ",", "," p ",") { print $1 ".pmtiles" }')

for previous in $earlier; do
    [ "${previous%%.*}" = "$major" ] || rebuild "${TAG} is a major version bump over v${previous}"

    assets=$(gh api "repos/{owner}/{repo}/releases/tags/v${previous}" --jq '.assets[].name')
    missing=$(grep -vxF -f <(printf '%s\n' "$assets") <<<"$wanted" || true)
    if [ -n "$missing" ]; then
        echo "v${previous} is missing $(echo $missing), trying the release before it" >&2
        continue
    fi

    if ! git diff --quiet "v${previous}" "$TAG" -- $RECIPE; then
        rebuild "the map recipe changed since v${previous}"
    fi
    [ "$(toolchain_pins "v${previous}")" = "$(toolchain_pins "$TAG")" ] ||
        rebuild "the map toolchain pins changed since v${previous}"

    echo "carrying the map packages forward from v${previous}" >&2
    echo "v${previous}"
    exit 0
done

rebuild "no earlier ${major}.x release carries every map package"
