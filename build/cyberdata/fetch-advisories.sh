#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
list="${here}/advisories.txt"
target="${here}/source/advisories"
site="https://www.cisa.gov"

mkdir -p "${target}"
rm -f "${target}"/*.json

while read -r id; do
    [ -n "${id}" ] || continue
    lower="$(printf '%s' "${id}" | tr '[:upper:]' '[:lower:]')"
    page="$(curl --fail --location --silent --show-error --max-time 60 -A 'Mozilla/5.0' "${site}/news-events/cybersecurity-advisories/${lower}")"
    link="$(printf '%s' "${page}" | grep -oE 'href="/sites/default/files/[^"]*\.stix_\.json"' | sed 's/^href="//; s/"$//' | sort | tail -1)"
    if [ -z "${link}" ]; then
        echo "error: ${id} publishes no STIX JSON on its page" >&2
        exit 1
    fi
    curl --fail --location --silent --show-error --max-time 60 -A 'Mozilla/5.0' --output "${target}/${lower}.json" "${site}${link}"
    echo "fetched ${id} from ${site}${link}"
done < "${list}"
