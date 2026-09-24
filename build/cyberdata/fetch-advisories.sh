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
    links="$(printf '%s' "${page}" | grep -oE 'href="/sites/default/files/[^"]*\.stix_\.json"' | sed 's/^href="//; s/"$//' | sort -u)"
    if [ -z "${links}" ]; then
        echo "error: ${id} publishes no STIX JSON on its page" >&2
        exit 1
    fi
    n=0
    for link in ${links}; do
        n=$((n + 1))
        suffix=""
        [ "${n}" -eq 1 ] || suffix="-${n}"
        curl --fail --location --silent --show-error --max-time 60 -A 'Mozilla/5.0' --output "${target}/${lower}${suffix}.json" "${site}${link}"
        echo "fetched ${id} from ${site}${link}"
    done
done < "${list}"
