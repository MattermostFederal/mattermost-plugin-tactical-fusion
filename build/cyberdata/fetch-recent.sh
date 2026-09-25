#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
recent="${here}/recent"
pages="${recent}/nvd"

api="https://services.nvd.nist.gov/rest/json/cves/2.0"
page_size=2000
max_days=120

days="${DAYS:-7}"
if ! [[ "${days}" =~ ^[0-9]+$ ]] || [ "${days}" -lt 1 ] || [ "${days}" -gt "${max_days}" ]; then
    echo "error: DAYS must be a whole number from 1 to ${max_days}, the widest window NVD answers; got '${days}'" >&2
    exit 2
fi

stamp_format="%Y-%m-%dT%H:%M:%S"
if date -u -d "1 day ago" >/dev/null 2>&1; then
    now="$(date -u +"${stamp_format}")"
    start="$(date -u -d "${days} days ago" +"${stamp_format}")"
else
    now="$(date -u +"${stamp_format}")"
    start="$(date -u -v-"${days}"d +"${stamp_format}")"
fi

headers=()
pause=6
if [ -n "${NVD_API_KEY:-}" ]; then
    headers=(--header "apiKey: ${NVD_API_KEY}")
    pause=1
fi

mkdir -p "${pages}"
rm -f "${pages}"/page-*.json "${recent}/window"

echo "fetching CVEs NVD published from ${start}Z to ${now}Z"

index=0
total=""
received=0
page=0

while [ -z "${total}" ] || [ "${index}" -lt "${total}" ]; do
    [ "${page}" -gt 0 ] && sleep "${pause}"

    target="${pages}/$(printf 'page-%04d.json' "${page}")"
    curl --fail --location --silent --show-error \
        --retry 4 --retry-delay "${pause}" --retry-all-errors \
        ${headers[@]+"${headers[@]}"} \
        --output "${target}" \
        "${api}?pubStartDate=${start}.000Z&pubEndDate=${now}.000Z&noRejected&resultsPerPage=${page_size}&startIndex=${index}"

    reported="$({ grep -o '"totalResults":[0-9]*' "${target}" || true; } | head -1 | cut -d: -f2)"
    if [ -z "${reported}" ]; then
        echo "error: NVD answered without a totalResults; the response began:" >&2
        head -c 300 "${target}" >&2
        echo >&2
        exit 1
    fi
    total="${reported}"

    count="$({ grep -o '"cve":{"id":"CVE-' "${target}" || true; } | wc -l | tr -d ' ')"
    if [ "${count}" -eq 0 ] && [ "${index}" -lt "${total}" ]; then
        echo "error: page ${page} held no CVEs with ${total} reported and ${received} received" >&2
        exit 1
    fi

    received=$((received + count))
    index=$((index + page_size))
    page=$((page + 1))
    echo "  page ${page}: ${count} CVEs (${received} of ${total})"
done

if [ "${received}" -ne "${total}" ]; then
    echo "error: NVD reported ${total} CVEs in the window and ${received} arrived" >&2
    exit 1
fi

printf 'NVD API, published %sZ to %sZ, %s CVEs\n' "${start}" "${now}" "${total}" > "${recent}/window"

echo "fetched ${received} CVEs into ${pages}"
