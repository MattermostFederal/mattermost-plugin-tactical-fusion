import React, {useCallback, useEffect, useRef, useState} from 'react';

import type {
    CyberDatabaseFile,
    CyberDatasetFile,
    CyberDatasetsResponse,
    CyberDirectory,
    CyberMissingDataset,
    CyberSkippedFile,
} from '../decorators/cyber/types';
import {pluginBaseUrl} from '../plugin_url';

const KIND_LABELS: Record<string, string> = {bundled: 'bundled', configured: 'your directory'};

const styles: Record<string, React.CSSProperties> = {
    root: {maxWidth: 900},
    table: {width: '100%', borderCollapse: 'collapse', fontSize: 13, margin: '0 0 12px'},
    th: {textAlign: 'left', fontWeight: 600, padding: '6px 8px 6px 0', borderBottom: '1px solid rgba(0, 0, 0, 0.16)', whiteSpace: 'nowrap'},
    td: {padding: '6px 8px 6px 0', borderBottom: '1px solid rgba(0, 0, 0, 0.08)', verticalAlign: 'top'},
    number: {textAlign: 'right', fontVariantNumeric: 'tabular-nums', whiteSpace: 'nowrap'},
    file: {fontFamily: 'monospace', fontSize: 12, whiteSpace: 'nowrap'},
    path: {fontFamily: 'monospace', fontSize: 12, overflowWrap: 'anywhere'},
    date: {whiteSpace: 'nowrap'},
    kind: {fontSize: 11, opacity: 0.72, whiteSpace: 'nowrap'},
    heading: {fontWeight: 600, margin: '12px 0 4px'},
    note: {margin: '0 0 8px', fontSize: 12, opacity: 0.72},
    error: {margin: '0 0 8px', color: '#c92a2a'},
    list: {margin: '0 0 8px', paddingLeft: 18, fontSize: 13},
};

const SIZE_UNITS = ['bytes', 'KB', 'MB', 'GB'];
const SIZE_STEP = 1024;

export function sizeText(bytes: number): string {
    let value = bytes;
    let unit = 0;
    while (value >= SIZE_STEP && unit < SIZE_UNITS.length - 1) {
        value /= SIZE_STEP;
        unit++;
    }
    return unit === 0 ? `${value} bytes` : `${value.toFixed(1)} ${SIZE_UNITS[unit]}`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return value !== null && typeof value === 'object' && !Array.isArray(value);
}

function strings(record: Record<string, unknown>, fields: string[]): boolean {
    return fields.every((field) => typeof record[field] === 'string');
}

function numbers(record: Record<string, unknown>, fields: string[]): boolean {
    return fields.every((field) => typeof record[field] === 'number');
}

function listOf<T>(value: unknown, valid: (entry: Record<string, unknown>) => boolean): T[] | null {
    if (!Array.isArray(value) || !value.every((entry) => isRecord(entry) && valid(entry))) {
        return null;
    }
    return value as T[];
}

export function asDatasets(body: unknown): CyberDatasetsResponse | null {
    if (!isRecord(body)) {
        return null;
    }

    const directories = listOf<CyberDirectory>(body.directories, (e) => strings(e, ['path', 'kind']));
    const datasets = listOf<CyberDatasetFile>(body.datasets, (e) =>
        strings(e, ['name', 'label', 'path', 'kind', 'countError', 'generated', 'source']) && numbers(e, ['size', 'records']));
    const databases = listOf<CyberDatabaseFile>(body.databases, (e) => strings(e, ['path', 'kind', 'type', 'built']) && numbers(e, ['size']));
    const missing = listOf<CyberMissingDataset>(body.missing, (e) => strings(e, ['name', 'label']));
    const skipped = listOf<CyberSkippedFile>(body.skipped, (e) => strings(e, ['path', 'reason']));
    const replaced = Array.isArray(body.replaced) && body.replaced.every((entry) => typeof entry === 'string') ? body.replaced as string[] : null;

    if (!directories || !datasets || !databases || !missing || !skipped || !replaced) {
        return null;
    }
    return {directories, datasets, databases, missing, replaced, skipped};
}

const STAMP = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2})(?::\d{2}(?:\.\d+)?)?Z$/;

export function stampText(generated: string): string {
    const match = STAMP.exec(generated);
    return match ? `${match[1]} ${match[2]} UTC` : generated;
}

function capitalized(label: string): string {
    return label.charAt(0).toUpperCase() + label.slice(1);
}

function fileName(path: string): string {
    return path.split('/').pop() ?? path;
}

const Datasets: React.FC<{datasets: CyberDatasetFile[]}> = ({datasets}) => {
    if (datasets.length === 0) {
        return <p style={styles.note}>{'No dataset is loaded.'}</p>;
    }

    return (
        <table
            style={styles.table}
            data-testid='cyber-datasets'
        >
            <thead>
                <tr>
                    <th style={styles.th}>{'Dataset'}</th>
                    <th style={styles.th}>{'File'}</th>
                    <th style={{...styles.th, ...styles.number}}>{'Records'}</th>
                    <th style={{...styles.th, ...styles.number}}>{'Size'}</th>
                    <th style={styles.th}>{'Generated'}</th>
                    <th style={styles.th}>{'Source'}</th>
                </tr>
            </thead>
            <tbody>
                {datasets.map((dataset) => (
                    <tr key={dataset.name}>
                        <td style={styles.td}>{capitalized(dataset.label || dataset.name)}</td>
                        <td style={styles.td}>
                            <span
                                style={styles.file}
                                title={dataset.path}
                            >{fileName(dataset.path)}</span>
                            <div style={styles.kind}>{KIND_LABELS[dataset.kind] ?? dataset.kind}</div>
                        </td>
                        <td style={{...styles.td, ...styles.number}}>
                            {dataset.countError === '' ? dataset.records.toLocaleString('en-US') : (
                                <span
                                    style={styles.error}
                                    title={dataset.countError}
                                >{'unknown'}</span>
                            )}
                        </td>
                        <td style={{...styles.td, ...styles.number}}>{sizeText(dataset.size)}</td>
                        <td style={{...styles.td, ...styles.date}}>{stampText(dataset.generated)}</td>
                        <td style={styles.td}>{dataset.source}</td>
                    </tr>
                ))}
            </tbody>
        </table>
    );
};

const Databases: React.FC<{databases: CyberDatabaseFile[]}> = ({databases}) => {
    if (databases.length === 0) {
        return null;
    }

    return (
        <>
            <p style={styles.heading}>{'Vendor IP databases'}</p>
            <table style={styles.table}>
                <tbody>
                    {databases.map((database) => (
                        <tr key={database.path}>
                            <td style={styles.td}>
                                <span
                                    style={styles.file}
                                    title={database.path}
                                >{fileName(database.path)}</span>
                                <div style={styles.kind}>{KIND_LABELS[database.kind] ?? database.kind}</div>
                            </td>
                            <td style={styles.td}>{database.type}</td>
                            <td style={styles.td}>{database.built === '' ? '' : `built ${database.built}`}</td>
                            <td style={{...styles.td, ...styles.number}}>{sizeText(database.size)}</td>
                        </tr>
                    ))}
                </tbody>
            </table>
        </>
    );
};

const CyberDatasets: React.FC = () => {
    const [data, setData] = useState<CyberDatasetsResponse | null>(null);
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);
    const live = useRef(true);

    useEffect(() => {
        live.current = true;
        return () => {
            live.current = false;
        };
    }, []);

    const refresh = useCallback(async () => {
        setLoading(true);
        setError('');
        try {
            const response = await fetch(`${pluginBaseUrl()}/api/v1/cyber/datasets`, {
                credentials: 'same-origin',
                headers: {'X-Requested-With': 'XMLHttpRequest'},
            });
            if (!response.ok) {
                throw new Error(`The server answered ${response.status}.`);
            }
            const parsed = asDatasets(await response.json());
            if (!parsed) {
                throw new Error('The server answered with something that is not a dataset list.');
            }
            if (live.current) {
                setData(parsed);
            }
        } catch (err) {
            if (live.current) {
                setError(err instanceof Error ? err.message : String(err));
            }
        } finally {
            if (live.current) {
                setLoading(false);
            }
        }
    }, []);

    useEffect(() => {
        refresh();
    }, [refresh]);

    return (
        <div
            style={styles.root}
            data-testid='cyber-datasets-admin'
        >
            {error !== '' && <p style={styles.error}>{`The dataset list could not be read. ${error}`}</p>}
            {loading && !data && <p style={styles.note}>{'Reading the datasets, and counting their records...'}</p>}
            {data && (
                <>
                    {data.directories.length > 0 && (
                        <p style={styles.note}>
                            {`Read from ${data.directories.map((dir) => `${dir.path} (${KIND_LABELS[dir.kind] ?? dir.kind})`).join(', ')}.`}
                        </p>
                    )}
                    <Datasets datasets={data.datasets}/>
                    <Databases databases={data.databases}/>
                    {data.missing.length > 0 && (
                        <p style={styles.note}>
                            {`Not installed: ${data.missing.map((missing) => `${missing.label || missing.name} (${missing.name}.tsv)`).join(', ')}.`}
                        </p>
                    )}
                    {data.replaced.length > 0 && (
                        <>
                            <p style={styles.heading}>{'Bundled files replaced by a file in your directory'}</p>
                            <ul style={styles.list}>
                                {data.replaced.map((path) => (
                                    <li
                                        key={path}
                                        style={styles.path}
                                    >{path}</li>
                                ))}
                            </ul>
                        </>
                    )}
                    {data.skipped.length > 0 && (
                        <>
                            <p style={styles.heading}>{'Skipped files'}</p>
                            <ul
                                style={styles.list}
                                data-testid='cyber-datasets-skipped'
                            >
                                {data.skipped.map((skipped) => (
                                    <li key={skipped.path}>
                                        <span style={styles.path}>{skipped.path}</span>
                                        {`: ${skipped.reason}`}
                                    </li>
                                ))}
                            </ul>
                        </>
                    )}
                </>
            )}
            <button
                type='button'
                className='btn btn-tertiary'
                onClick={refresh}
                disabled={loading}
            >{loading ? 'Refreshing...' : 'Refresh'}</button>
        </div>
    );
};

export default CyberDatasets;
