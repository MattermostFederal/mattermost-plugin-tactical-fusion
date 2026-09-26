#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source_dir="${here}/source"
pins="${here}/pins.env"
manifest="${SOURCES_MANIFEST:-${source_dir}/SOURCES.sha256}"
nvd_attempts=3

pinned() {
    local name="$1" value
    value="$(sed -n "s/^${name}=//p" "${pins}" | tail -1)"
    if ! [[ "${value}" =~ ^[0-9a-f]{40}$ ]]; then
        echo "error: ${name} in ${pins} is not a full 40 character commit SHA: '${value}'" >&2
        exit 1
    fi
    printf '%s' "${value}"
}

attack_commit="$(pinned ATTACK_STIX_DATA_COMMIT)"
mappings_commit="$(pinned MAPPINGS_EXPLORER_COMMIT)"
misp_commit="$(pinned MISP_WARNINGLISTS_COMMIT)"

mkdir -p "${source_dir}" "${source_dir}/nvd"
cp "${pins}" "${source_dir}/PINS"

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

sha256_of() {
    shasum -a 256 "$1" | cut -d' ' -f1
}

fetch_nvd_year() {
    local year="$1" attempt meta want="" got=""
    local base="https://nvd.nist.gov/feeds/json/cve/2.0/nvdcve-2.0-${year}"
    local target="${source_dir}/nvd/nvdcve-2.0-${year}.json"

    for attempt in $(seq 1 "${nvd_attempts}"); do
        echo "fetching nvd/${year} (attempt ${attempt})"
        meta="$(curl --fail --location --silent --show-error "${base}.meta" | tr -d '\r')" || meta=""
        want="$(printf '%s\n' "${meta}" | sed -n 's/^sha256://p' | tr 'A-F' 'a-f')"
        if ! [[ "${want}" =~ ^[0-9a-f]{64}$ ]]; then
            echo "nvd/${year}.meta could not be read or carries no sha256"
            continue
        fi
        if ! curl --fail --location --silent --show-error "${base}.json.gz" | gunzip > "${target}"; then
            echo "nvd/${year} could not be downloaded"
            continue
        fi
        got="$(sha256_of "${target}")"
        if [ "${got}" = "${want}" ]; then
            return 0
        fi
        echo "nvd/${year} does not match its .meta (NVD may have republished mid-fetch)"
    done
    echo "error: nvd/${year} never matched a sha256 read from its .meta in ${nvd_attempts} attempts" >&2
    echo "  expected ${want:-no sha256}" >&2
    echo "  got      ${got:-no download}" >&2
    exit 1
}

attack_base="https://raw.githubusercontent.com/mitre-attack/attack-stix-data/${attack_commit}"
fetch "${attack_base}/enterprise-attack/enterprise-attack.json" "enterprise-attack.json"
fetch "${attack_base}/mobile-attack/mobile-attack.json" "mobile-attack.json"

echo "fetching cwe-1000.csv"
curl --fail --location --silent --show-error --output "${source_dir}/cwe-1000.csv.zip" \
    "https://cwe.mitre.org/data/csv/1000.csv.zip"
unzip -p "${source_dir}/cwe-1000.csv.zip" > "${source_dir}/cwe-1000.csv"

echo "fetching capec-1000.csv"
curl --fail --location --silent --show-error --output "${source_dir}/capec-1000.csv.zip" \
    "https://capec.mitre.org/data/csv/1000.csv.zip"
unzip -p "${source_dir}/capec-1000.csv.zip" > "${source_dir}/capec-1000.csv"

mappings_path="mappings/kev/attack-16.1/kev-07.28.2025"
for domain in enterprise mobile; do
    fetch "https://raw.githubusercontent.com/center-for-threat-informed-defense/mappings-explorer/${mappings_commit}/${mappings_path}/${domain}/kev-07.28.2025_attack-16.1-${domain}.json" \
        "kev-attack-${domain}.json"
done

fetch "https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json" \
    "known_exploited_vulnerabilities.json"

rm -f "${source_dir}"/nvd/nvdcve-2.0-*.json
for year in $(seq 2002 "$(date -u +%Y)"); do
    fetch_nvd_year "${year}"
done

fetch_gz "https://epss.empiricalsecurity.com/epss_scores-current.csv.gz" \
    "epss_scores-current.csv"

fetch_gz "https://iptoasn.com/data/ip2asn-combined.tsv.gz" "ip2asn-combined.tsv"

fetch "https://check.torproject.org/torbulkexitlist" "tor-exits.txt"

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

mkdir -p "$(dirname "${manifest}")"
(cd "${source_dir}" && shasum -a 256 \
    PINS enterprise-attack.json mobile-attack.json cwe-1000.csv.zip capec-1000.csv.zip \
    kev-attack-enterprise.json kev-attack-mobile.json known_exploited_vulnerabilities.json \
    nvd/nvdcve-2.0-*.json epss_scores-current.csv ip2asn-combined.tsv tor-exits.txt \
    misp-warninglists/COMMIT misp-warninglists/*.json) > "${manifest}"

echo "sources are in ${source_dir}; their digests are in ${manifest}"
