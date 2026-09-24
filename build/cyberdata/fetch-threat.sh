#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
feeds="${here}/source/threat"

mkdir -p "${feeds}"

fetch() {
    local name="$1" url="$2" target="$3"
    local staging
    staging="$(mktemp "${feeds}/.${target}.XXXXXX")"
    if curl --fail --location --silent --show-error --max-time 900 --output "${staging}" "${url}"; then
        mv "${staging}" "${feeds}/${target}"
        echo "fetched ${name} ($(wc -c < "${feeds}/${target}" | tr -d ' ') bytes)"
    else
        rm -f "${staging}"
        echo "warning: could not fetch ${name}; the build uses the other feeds" >&2
    fi
}

fetch "abuse.ch ThreatFox full export" "https://threatfox.abuse.ch/export/csv/full/" "threatfox-full.zip"
fetch "abuse.ch MalwareBazaar full export" "https://bazaar.abuse.ch/export/csv/full/" "malwarebazaar-full.zip"
fetch "abuse.ch Feodo Tracker blocklist" "https://feodotracker.abuse.ch/downloads/ipblocklist.csv" "feodo-ipblocklist.csv"
fetch "Tor Project exit list" "https://check.torproject.org/torbulkexitlist" "tor-exits.txt"

date -u +%Y-%m-%d > "${feeds}/fetched"
