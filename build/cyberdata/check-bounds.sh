#!/usr/bin/env bash

set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tree="$(cd "${here}/../.." && pwd)"
out="${here}/out"
baseline="${CYBER_BASELINE:-HEAD}"
allow_shrink="${CYBER_ALLOW_SHRINK:-0}"
dbip_min_bytes=50000000

failures=0

fail() {
    echo "error: $*" >&2
    failures=$((failures + 1))
}

read_plain() {
    case "$1" in
        *.gz) gunzip -c ;;
        *) cat ;;
    esac
}

count_rows() {
    awk -v p="$1" '!/^#/ && (p == "" || index($0, p)) { n++ } END { print n + 0 }'
}

rows_of() {
    local path="$1" pattern="${2:-}"
    read_plain "${path}" < "${tree}/${path}" | count_rows "${pattern}"
}

committed_rows_of() {
    local path="$1" pattern="${2:-}"
    git -C "${tree}" rev-parse --verify --quiet "${baseline}^{commit}" > /dev/null || return 1
    if ! git -C "${tree}" cat-file -e "${baseline}:${path}" 2>/dev/null; then
        return 0
    fi
    git -C "${tree}" show "${baseline}:${path}" | read_plain "${path}" | count_rows "${pattern}"
}

check() {
    local label="$1" path="$2" floor="$3" max_shrink_pct="$4" pattern="${5:-}"
    local now was least

    if [ ! -f "${tree}/${path}" ]; then
        fail "${label}: ${path} was not generated"
        return
    fi
    if ! now="$(rows_of "${path}" "${pattern}")"; then
        fail "${label}: ${path} could not be read"
        return
    fi
    if ! was="$(committed_rows_of "${path}" "${pattern}")"; then
        fail "${label}: ${path} could not be read at ${baseline}"
        return
    fi

    if [ "${now}" -lt "${floor}" ]; then
        fail "${label}: ${now} rows, below the floor of ${floor}; the download was truncated or is not what it should be"
        return
    fi
    if [ -n "${was}" ] && [ "${was}" -gt 0 ] && [ "${allow_shrink}" != "1" ]; then
        least=$((was * (100 - max_shrink_pct) / 100))
        if [ "${now}" -lt "${least}" ]; then
            fail "${label}: ${now} rows against ${was} at ${baseline}, a shrink of more than ${max_shrink_pct}%; set CYBER_ALLOW_SHRINK=1 once the upstream is confirmed"
            return
        fi
    fi
    echo "${label}: ${now} rows (${was:-none} at ${baseline})"
}

check "ATT&CK catalog" server/decorators/cyber/data/attack.tsv 500 5
check "CWE catalog" server/decorators/cyber/data/cwe.tsv 500 5
check "ATT&CK detail" assets/cyber/attackdetail.tsv 500 5
check "CWE detail" assets/cyber/cwedetail.tsv 500 5
check "CAPEC" assets/cyber/capec.tsv 300 5
check "CVE to ATT&CK" assets/cyber/cveattack.tsv 100 5
check "KEV" assets/cyber/kev.tsv 1000 2
check "KEV CVE slice" assets/cyber/cve.tsv 1000 2
check "KEV CVE detail slice" assets/cyber/cvedetail.tsv 1000 2
check "EPSS" assets/cyber/epss.tsv.gz 250000 5
check "IPtoASN" assets/cyber/ip.tsv.gz 400000 10
check "Tor exits" assets/cyber/netlists.tsv.gz 500 50 '"source":"Tor Project"'
check "MISP network warninglists" assets/cyber/netlists.tsv.gz 50000 5 '"source":"MISP warninglist"'
check "MISP hash warninglists" assets/cyber/hashlists.tsv 50 5

if [ ! -f "${out}/cve.tsv" ]; then
    fail "full CVE dataset: ${out}/cve.tsv is missing"
elif ! cve_full="$(count_rows "" < "${out}/cve.tsv")"; then
    fail "full CVE dataset: ${out}/cve.tsv could not be read"
elif [ "${cve_full}" -lt 300000 ]; then
    fail "full CVE dataset: ${cve_full} rows in ${out}/cve.tsv, below the floor of 300000"
else
    echo "full CVE dataset: ${cve_full} rows"
fi

if [ -f "${out}/dbip-city-lite.mmdb" ]; then
    dbip_bytes="$(wc -c < "${out}/dbip-city-lite.mmdb" | tr -d ' ')"
    if [ "${dbip_bytes}" -lt "${dbip_min_bytes}" ]; then
        fail "DB-IP City Lite: ${dbip_bytes} bytes, below the floor of ${dbip_min_bytes}"
    else
        echo "DB-IP City Lite: ${dbip_bytes} bytes"
    fi
else
    fail "DB-IP City Lite: ${out}/dbip-city-lite.mmdb is missing"
fi

if [ "${failures}" -gt 0 ]; then
    echo "${failures} cyber dataset(s) out of bounds; nothing here may ship" >&2
    exit 1
fi
echo "cyber datasets are within bounds"
