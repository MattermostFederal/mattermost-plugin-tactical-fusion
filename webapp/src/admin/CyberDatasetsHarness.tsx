import React from 'react';

import CyberDatasets from './CyberDatasets';

export type DatasetsReply = 'ok' | 'empty' | 'refused' | 'not-a-list' | 'count-failed';

const LOADED = {
    directories: [
        {path: '/plugins/tactical-fusion/assets/cyber', kind: 'bundled'},
        {path: '/mattermost/data/cyber-datasets', kind: 'configured'},
    ],
    datasets: [
        {name: 'cve', label: 'vulnerability', path: '/mattermost/data/cyber-datasets/cve.tsv', kind: 'configured', size: 185738016, records: 396474, countError: '', generated: '2026-09-23T20:57:13Z'},
        {name: 'kev', label: 'known exploited vulnerabilities', path: '/plugins/tactical-fusion/assets/cyber/kev.tsv', kind: 'bundled', size: 276258, records: 1721, countError: '', generated: '2026-09-24T01:00:44Z'},
    ],
    databases: [
        {path: '/mattermost/data/cyber-datasets/dbip-city-lite.mmdb', kind: 'configured', type: 'DBIP-City-Lite', built: '2026-09-01', size: 127339927},
    ],
    missing: [{name: 'watchlist', label: 'watchlist'}],
    replaced: ['/plugins/tactical-fusion/assets/cyber/advisory.tsv'],
    skipped: [{path: '/mattermost/data/cyber-datasets/notes.tsv', reason: 'a file in the cyber dataset directory is not a dataset this build reads (TF-21003)'}],
};

function bodyFor(reply: DatasetsReply): unknown {
    switch (reply) {
    case 'empty':
        return {...LOADED, datasets: [], databases: [], replaced: [], skipped: [], directories: []};
    case 'not-a-list':
        return {...LOADED, datasets: 'cve'};
    case 'count-failed':
        return {...LOADED, datasets: [{...LOADED.datasets[0], records: 0, countError: 'The records could not be counted: read failed (TF-21001)'}]};
    }
    return LOADED;
}

const CyberDatasetsHarness: React.FC<{reply?: DatasetsReply}> = ({reply = 'ok'}) => {
    React.useState(() => {
        globalThis.fetch = (async () => {
            if (reply === 'refused') {
                return {status: 403, ok: false} as Response;
            }
            return {status: 200, ok: true, json: async () => bodyFor(reply)} as unknown as Response;
        }) as typeof globalThis.fetch;
        return 0;
    });

    return <CyberDatasets/>;
};

export default CyberDatasetsHarness;
