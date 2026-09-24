#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
out="${here}/out"
target="${out}/dbip-city-lite.mmdb"
base="https://download.db-ip.com/free"
mmdb_metadata_marker=$'\xab\xcd\xefMaxMind.com'

this_month="$(date -u +%Y-%m)"
if date -u -d "1 month ago" >/dev/null 2>&1; then
    last_month="$(date -u -d "$(date -u +%Y-%m-15) 1 month ago" +%Y-%m)"
else
    last_month="$(date -u -v15d -v-1m +%Y-%m)"
fi

mkdir -p "${out}"
staging="$(mktemp "${out}/.dbip-city-lite.XXXXXX")"
trap 'rm -f "${staging}"' EXIT

fetched=""
for month in "${this_month}" "${last_month}"; do
    if curl --fail --location --silent --show-error "${base}/dbip-city-lite-${month}.mmdb.gz" | gunzip > "${staging}" 2>/dev/null; then
        fetched="${month}"
        break
    fi
    echo "DB-IP City Lite for ${month} is not published yet"
done

if [ -z "${fetched}" ]; then
    echo "error: could not fetch DB-IP City Lite for ${this_month} or ${last_month}" >&2
    exit 1
fi

if ! LC_ALL=C grep -q -a "${mmdb_metadata_marker}" "${staging}"; then
    echo "error: what DB-IP sent is not a MaxMind-format database" >&2
    exit 1
fi

mv "${staging}" "${target}"
trap - EXIT
echo "fetched DB-IP City Lite ${fetched} into ${target} ($(wc -c < "${target}" | tr -d ' ') bytes)"
