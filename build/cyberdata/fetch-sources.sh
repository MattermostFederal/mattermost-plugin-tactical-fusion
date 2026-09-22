#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source_dir="${here}/source"
lock="${here}/sources.lock"

mkdir -p "${source_dir}" "${source_dir}/nvd"

fetch() {
    local url="$1" target="$2"

    echo "fetching ${target}"
    curl --fail --location --silent --show-error --output "${source_dir}/${target}" "${url}"
}

fetch_gz() {
    local url="$1" target="$2"

    echo "fetching ${target}"
    curl --fail --location --silent --show-error "${url}" | gunzip > "${source_dir}/${target}"
}

fetch "https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack.json" \
    "enterprise-attack.json"

echo "fetching cwe-1000.csv"
curl --fail --location --silent --show-error --output "${source_dir}/cwe-1000.csv.zip" \
    "https://cwe.mitre.org/data/csv/1000.csv.zip"
unzip -p "${source_dir}/cwe-1000.csv.zip" > "${source_dir}/cwe-1000.csv"

fetch "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json" \
    "known_exploited_vulnerabilities.json"

for year in $(seq 2002 "$(date -u +%Y)"); do
    echo "fetching nvd/${year}"
    curl --fail --location --silent --show-error \
        "https://nvd.nist.gov/feeds/json/cve/2.0/nvdcve-2.0-${year}.json.gz" \
        | gunzip > "${source_dir}/nvd/nvdcve-2.0-${year}.json"
done

fetch_gz "https://epss.empiricalsecurity.com/epss_scores-current.csv.gz" \
    "epss_scores-current.csv"

fetch_gz "https://iptoasn.com/data/ip2asn-combined.tsv.gz" "ip2asn-combined.tsv"

if [ -f "${lock}" ]; then
    echo "verifying against sources.lock"
    (cd "${source_dir}" && shasum -a 256 -c "${lock}")
else
    echo "no sources.lock yet; writing one from what was just fetched"
    (cd "${source_dir}" && shasum -a 256 \
        enterprise-attack.json cwe-1000.csv known_exploited_vulnerabilities.json \
        epss_scores-current.csv ip2asn-combined.tsv > "${lock}")
fi

echo "sources are in ${source_dir}"
