#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source_dir="${here}/source"
lock="${SOURCES_LOCK:-${here}/sources.lock}"

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

fetch "https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/mobile-attack/mobile-attack.json" \
    "mobile-attack.json"

echo "fetching cwe-1000.csv"
curl --fail --location --silent --show-error --output "${source_dir}/cwe-1000.csv.zip" \
    "https://cwe.mitre.org/data/csv/1000.csv.zip"
unzip -p "${source_dir}/cwe-1000.csv.zip" > "${source_dir}/cwe-1000.csv"

echo "fetching capec-1000.csv"
curl --fail --location --silent --show-error --output "${source_dir}/capec-1000.csv.zip" \
    "https://capec.mitre.org/data/csv/1000.csv.zip"
unzip -p "${source_dir}/capec-1000.csv.zip" > "${source_dir}/capec-1000.csv"

mappings_commit="e51d7f595db675df064ffc2b5c35c88f98eb3688"
mappings_path="mappings/kev/attack-16.1/kev-07.28.2025"
for domain in enterprise mobile; do
    fetch "https://raw.githubusercontent.com/center-for-threat-informed-defense/mappings-explorer/${mappings_commit}/${mappings_path}/${domain}/kev-07.28.2025_attack-16.1-${domain}.json" \
        "kev-attack-${domain}.json"
done

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

fetch "https://check.torproject.org/torbulkexitlist" "tor-exits.txt"

misp_repo="https://github.com/MISP/misp-warninglists"
misp_commit="$(git ls-remote "${misp_repo}.git" refs/heads/main | cut -f1)"
if [ -z "${misp_commit}" ]; then
    echo "error: could not resolve the MISP warninglists main branch" >&2
    exit 1
fi
misp_dir="${source_dir}/misp-warninglists"
misp_tree="$(mktemp -d)"
trap 'rm -rf "${misp_tree}"' EXIT
echo "fetching MISP warninglists at ${misp_commit}"
curl --fail --location --silent --show-error "https://codeload.github.com/MISP/misp-warninglists/tar.gz/${misp_commit}" \
    | tar xz -C "${misp_tree}" --strip-components=1
rm -rf "${misp_dir}"
mkdir -p "${misp_dir}"
while read -r list; do
    [ -n "${list}" ] || continue
    if [ ! -f "${misp_tree}/lists/${list}/list.json" ]; then
        echo "error: warninglist ${list} is not in MISP warninglists ${misp_commit}" >&2
        exit 1
    fi
    cp "${misp_tree}/lists/${list}/list.json" "${misp_dir}/${list}.json"
done < "${here}/warninglists.txt"
printf '%s\n' "${misp_commit}" > "${misp_dir}/COMMIT"

if [ -f "${lock}" ]; then
    echo "verifying against ${lock}"
    (cd "${source_dir}" && shasum -a 256 -c "${lock}")
else
    echo "no ${lock} yet; writing one from what was just fetched"
    (cd "${source_dir}" && shasum -a 256 \
        enterprise-attack.json mobile-attack.json cwe-1000.csv capec-1000.csv \
        kev-attack-enterprise.json kev-attack-mobile.json known_exploited_vulnerabilities.json \
        epss_scores-current.csv ip2asn-combined.tsv tor-exits.txt \
        misp-warninglists/COMMIT misp-warninglists/*.json > "${lock}")
fi

echo "sources are in ${source_dir}"
