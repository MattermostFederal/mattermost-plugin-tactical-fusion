import React from 'react';

import {_resetForTesting as resetCyber} from './cyber';
import CyberHover from './CyberHover';
import CyberPanel from './CyberPanel';
import type {CyberCredit, CyberSection} from './types';

import {
    _resetForTesting as resetSelection,
    initRhs,
    subscribe,
} from '../selection';
import type {Selection} from '../selection';

import type {CyberPayload} from './index';

type Reply = 'found' | 'weakness' | 'technique' | 'address' | 'long' | 'bare' | 'status' | 'rejected' | 'failed' | 'hold';

const HEADLINE = '10.0 Critical, in KEV';
const SUMMARY = 'Remote code execution in a logging library.';

const PUBLISHED_QUERY = 'a=&dtg=101015ZDEC21&t=1639131300000&z=Z';

const FOUND = {
    kind: 'cve',
    value: 'CVE-2021-44228',
    title: 'CVE-2021-44228',
    headline: HEADLINE,
    summary: SUMMARY,
    status: '',
    rows: [
        {label: 'CVSS', value: '10.0 Critical', query: ''},
        {label: 'Published', value: '2021-12-10 10:15 UTC', query: PUBLISHED_QUERY},
    ],
    related: [{kind: 'cwe', value: 'CWE-502', label: 'CWE-502 Deserialization of Untrusted Data'}],
    watchlist: [{
        verdict: 'malicious',
        source: 'internal',
        note: 'seen beaconing',
        updated: '2026-08-01',
        known: true,
    }],
    datasets: [
        {name: 'cve', label: 'vulnerability', present: true, generated: '2026-09-01T00:00:00Z'},
        {name: 'ip', label: 'IP address', present: false, generated: ''},
    ],
    score: '10.0',
    severity: 'critical',
    exploited: true,
    vector: [
        {metric: 'Attack vector', value: 'Network', severe: true},
        {metric: 'User interaction', value: 'Required', severe: false},
    ],
    affected: ['Apache Software Foundation Apache Log4j2: from 2.0-beta9 before 2.15.0'],
    configurations: [
        'apache log4j: from 2.0 before 2.3.1, from 2.4 before 2.12.2',
        'siemens sppa-t3000 firmware: all versions (on siemens sppa-t3000)',
    ],
    sections: [] as CyberSection[],
    credits: [] as CyberCredit[],
    glance: {subtitle: '', summary: '', tags: [] as string[], facts: [] as string[], status: ''},
    references: [
        {url: 'http://packetstormsecurity.com/files/165225/Apache-Log4j2-2.14.1-Remote-Code-Execution.html', tags: 'Third Party Advisory, VDB Entry'},
        {url: 'https://lists.debian.org/debian-lts-announce/2021/12/msg00007.html', tags: 'Mailing List'},
        {url: 'https://logging.apache.org/log4j/2.x/security.html', tags: 'Vendor Advisory, Patch'},
        // eslint-disable-next-line no-script-url
        {url: 'javascript:alert(1)', tags: 'Exploit'},
    ],
};

const BARE = {
    ...FOUND,
    summary: '',
    score: '',
    severity: '',
    exploited: false,
    vector: [],
    rows: [],
    related: [],
    watchlist: [],
    affected: [],
    configurations: [],
    references: [],
};

const NO_DATASET = {
    ...BARE,
    headline: 'No vulnerability dataset is installed.',
    status: 'No vulnerability dataset is installed.',
    datasets: [{name: 'cve', label: 'vulnerability', present: false, generated: ''}],
};

const WEAKNESS = {
    ...FOUND,
    kind: 'cwe',
    value: 'CWE-79',
    title: 'Cross-site Scripting',
    score: '',
    severity: '',
    exploited: false,
    vector: [],
    affected: [],
    configurations: [],
    references: [],
    glance: {
        subtitle: 'CWE-79 · Base · Stable',
        summary: 'The product does not neutralize input placed in a web page.',
        tags: ['Confidentiality', 'Integrity'],
        facts: ['12 mitigations', '20 observed examples'],
        status: '',
    },
    sections: [
        {
            title: 'Mitigations',
            items: [
                {head: 'Implementation, Output Encoding (effectiveness high)', text: 'Encode it.', kind: '', value: '', url: ''},
            ],
        },
        {
            title: 'Observed examples',
            items: [
                {head: 'CVE-2021-44228', text: 'An invented example.', kind: 'cve', value: 'CVE-2021-44228', url: ''},
                {head: '[REF-1]', text: 'A citation.', kind: '', value: '', url: ''},
            ],
        },
    ],
};

const TECHNIQUE = {
    ...WEAKNESS,
    kind: 'attack',
    value: 'T1059.001',
    title: 'PowerShell',
    watchlist: [{verdict: 'suspicious', source: 'internal', note: '', updated: '2026-08-01', known: true}],
    glance: {
        subtitle: 'T1685 · Technique',
        summary: 'Adversaries may disable security tools.',
        tags: ['Defense Impairment'],
        facts: ['Windows, Linux'],
        status: 'Revoked by MITRE, replaced by T1685 Disable or Modify Tools',
    },
    sections: [
        {
            title: 'Procedure examples',
            items: [
                {head: 'G0007 APT28 (group)', text: 'APT28 used PowerShell.', kind: '', value: '', url: 'https://attack.mitre.org/groups/G0007'},
                // eslint-disable-next-line no-script-url
                {head: 'Bad Scheme', text: 'Never a link.', kind: '', value: '', url: 'javascript:alert(1)'},
            ],
        },
    ],
};

const ADDRESS = {
    ...WEAKNESS,
    kind: 'ip',
    value: '8.8.8.8',
    title: '8.8.8.8',
    headline: 'AS15169 GOOGLE, US',
    glance: {subtitle: 'AS15169 GOOGLE', summary: '', tags: ['Global'], facts: ['Mountain View, California, US'], status: ''},
    sections: [],
    rows: [{label: 'City', value: 'Mountain View', query: ''}],
    credits: [
        {text: 'IP Geolocation by DB-IP', url: 'https://db-ip.com'},
        // eslint-disable-next-line no-script-url
        {text: 'A vendor with a bad address', url: 'javascript:alert(1)'},
    ],
};

const LONG = {
    ...FOUND,
    summary: `${SUMMARY} ${'The rest of a long description. '.repeat(20)}`.trim(),
};

function baseFor(reply: Reply): typeof FOUND {
    if (reply === 'weakness') {
        return WEAKNESS;
    }
    if (reply === 'technique') {
        return TECHNIQUE;
    }
    if (reply === 'address') {
        return ADDRESS;
    }
    if (reply === 'long') {
        return LONG;
    }
    if (reply === 'status') {
        return NO_DATASET;
    }
    if (reply === 'bare') {
        return BARE;
    }

    return FOUND;
}

function bodyFor(reply: Reply, kind: string, value: string): unknown {
    const base = baseFor(reply);

    return {...base, kind, value, title: base.title === base.value ? value : base.title};
}

function paramOf(url: string, name: string): string {
    return new URLSearchParams(url.slice(url.indexOf('?'))).get(name) ?? '';
}

let onRequest: (() => void) | null = null;
let setups = 0;

const testStore = {
    getState: () => ({}),
    dispatch: () => undefined,
    subscribe: () => () => undefined,
    replaceReducer: () => undefined,
};

const CyberHarness: React.FC<{
    surface: 'panel' | 'hover';
    payload: CyberPayload;
    second?: CyberPayload;
    reply?: Reply;
}> = ({surface, payload, second, reply = 'found'}) => {
    const [requests, setRequests] = React.useState(0);
    const [shown, setShown] = React.useState(payload);

    React.useState(() => {
        setups += 1;
        resetCyber();
        resetSelection();

        initRhs(testStore as never, null);

        onRequest = () => setRequests((n) => n + 1);

        const real = globalThis.fetch;
        globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
            const url = String(input);

            if (!url.includes('/api/v1/cyber')) {
                return real(input, init);
            }

            onRequest?.();
            if (reply === 'hold') {
                return new Promise<Response>(() => undefined);
            }
            if (reply === 'failed') {
                throw new Error('offline');
            }
            if (reply === 'rejected') {
                return {status: 400, ok: false} as Response;
            }

            return {
                status: 200,
                ok: true,
                json: async () => bodyFor(reply, paramOf(url, 'k'), paramOf(url, 'v')),
            } as unknown as Response;
        }) as typeof globalThis.fetch;

        return setups;
    });

    const [selection, setSelected] = React.useState<Selection | null>(null);
    React.useEffect(() => subscribe(setSelected), []);

    const Surface = surface === 'panel' ? CyberPanel : CyberHover;

    return (
        <div>
            <Surface payload={shown}/>
            <p data-testid='requests'>{requests}</p>
            <p data-testid='selection'>
                {selection ? `${selection.type}:${JSON.stringify(selection.payload)}` : 'none'}
            </p>
            {second && (
                <button
                    type='button'
                    onClick={() => setShown(second)}
                >{'Show the second'}</button>
            )}
        </div>
    );
};

export default CyberHarness;
