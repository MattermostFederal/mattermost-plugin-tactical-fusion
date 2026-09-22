export const PANEL_TITLE = 'Aviation report';

const STATION = /^[A-Z][A-Z0-9]{3}$/;
const TIME_GROUP = /^\d{6}Z$/;
const FAA_LOCATION = /^![A-Z]{3}$/;
const ICAO_NUMBER = /^[A-Z]\d{4}\/\d{2}$/;
const MODIFIERS = new Set(['COR', 'AMD']);

function stationAfter(words: string[], from: number): string {
    let index = from;
    while (index < words.length && MODIFIERS.has(words[index])) {
        index += 1;
    }
    return index < words.length && STATION.test(words[index]) ? words[index] : '';
}

function heading(kind: string, station: string): string {
    return station === '' ? kind : `${kind} ${station}`;
}

export function headingFromSource(source: string): string {
    const words = source.trim().split(/\s+/);
    const first = words[0] ?? '';

    if (first === 'METAR' || first === 'SPECI' || first === 'TAF') {
        return heading(first, stationAfter(words, 1));
    }
    if (FAA_LOCATION.test(first)) {
        return heading('NOTAM', first.slice(1));
    }
    if (ICAO_NUMBER.test(first)) {
        const at = words.indexOf('A)');
        return heading('NOTAM', at === -1 ? '' : stationAfter(words, at + 1));
    }
    if (STATION.test(first) && TIME_GROUP.test(words[1] ?? '')) {
        return heading('METAR', first);
    }

    return PANEL_TITLE;
}
