import React, {useCallback, useEffect, useRef, useState} from 'react';

import {PACKAGE_NAME} from '../decorators/location/map/basemap';
import {pluginBaseUrl} from '../plugin_url';

/**
 * The System Console control for detail map packages.
 *
 * A `custom` setting, because it is the only setting type that can carry a
 * file: every other type is a string, a number or a switch.
 *
 * What it manages is the SAME directory an operator can copy a file into, and
 * that is not a coincidence to be tidied away later. An archive is read by byte
 * range for the life of the install, so it has to sit somewhere the server can
 * open; there is no plugin storage API that can serve a range. Upload and copy
 * are therefore two doors into one room, and this control says so rather than
 * presenting itself as a store of its own.
 */

const MAX_UPLOAD_MB = 512;

const styles: Record<string, React.CSSProperties> = {
    root: {maxWidth: 640},
    list: {margin: '0 0 12px', padding: 0, listStyle: 'none'},
    row: {
        display: 'flex',
        alignItems: 'center',
        gap: 8,
        padding: '6px 0',
        borderBottom: '1px solid rgba(0, 0, 0, 0.08)',
    },
    name: {flex: 1, fontFamily: 'monospace'},
    empty: {margin: '0 0 12px', opacity: 0.72},
    bundled: {opacity: 0.6, fontSize: 12},
    note: {margin: '8px 0 0', fontSize: 12, opacity: 0.72},
    error: {margin: '8px 0 0', color: '#c92a2a'},
    busy: {margin: '8px 0 0', opacity: 0.72},
};

function names(value: unknown): string[] {
    return Array.isArray(value) ? value.filter((name): name is string => typeof name === 'string') : [];
}

const PackageUploader: React.FC = () => {
    const [packages, setPackages] = useState<string[]>([]);
    const [removable, setRemovable] = useState<string[]>([]);
    const [error, setError] = useState<string | null>(null);
    const [busy, setBusy] = useState<string | null>(null);
    const file = useRef<HTMLInputElement | null>(null);
    const live = useRef(true);

    useEffect(() => {
        live.current = true;

        return () => {
            live.current = false;
        };
    }, []);

    const refresh = useCallback(async () => {
        try {
            const response = await fetch(`${pluginBaseUrl()}/api/v1/packages`, {
                credentials: 'same-origin',
                headers: {'X-Requested-With': 'XMLHttpRequest'},
            });
            if (!response.ok) {
                return;
            }

            const body = await response.json() as {packages?: unknown; removable?: unknown};
            if (live.current && Array.isArray(body.packages)) {
                setPackages(names(body.packages));
                setRemovable(names(body.removable));
            }
        } catch {
            // The list is a convenience here; the directory is the truth, and an
            // admin who cannot reach this route has larger problems than a
            // stale list.
        }
    }, []);

    useEffect(() => {
        refresh().catch(() => {
            // refresh never rejects.
        });
    }, [refresh]);

    const upload = useCallback(async (chosen: File) => {
        // The name comes from the FILE, not from a field, because the name is
        // what the map is keyed on: asking for it separately would let an admin
        // upload indopacom-hawaii.pmtiles under a different name and then
        // wonder why the area they built is not the area they see.
        const name = chosen.name.replace(/\.pmtiles$/i, '');
        if (!PACKAGE_NAME.test(name)) {
            setError(`"${chosen.name}" is not named <command>-<area>.pmtiles, such as indopacom-hawaii.pmtiles.`);
            return;
        }
        if (chosen.size > MAX_UPLOAD_MB * 1024 * 1024) {
            setError(`${chosen.name} is larger than ${MAX_UPLOAD_MB} MB. Copy it into the package directory instead.`);
            return;
        }

        setError(null);
        setBusy(name);

        try {
            const response = await fetch(`${pluginBaseUrl()}/api/v1/packages/${name}`, {
                method: 'POST',
                credentials: 'same-origin',

                // Mattermost accepts session-cookie authentication only for
                // requests that could not have been a cross-site form post, so a
                // write without this header is rejected as a CSRF attempt.
                headers: {'X-Requested-With': 'XMLHttpRequest'},
                body: chosen,
            });

            const body = await response.json() as {packages?: unknown; message?: unknown};
            if (!response.ok) {
                setError(typeof body.message === 'string' ? body.message : 'The upload was refused.');
                return;
            }

            if (Array.isArray(body.packages)) {
                setPackages(body.packages.filter((n): n is string => typeof n === 'string'));
            }
        } catch {
            setError('The upload could not be completed. A large package may exceed what this server accepts; copy it into the package directory instead.');
        } finally {
            if (live.current) {
                setBusy(null);
            }
        }
    }, []);

    const remove = useCallback(async (name: string) => {
        setError(null);
        setBusy(name);

        try {
            const response = await fetch(`${pluginBaseUrl()}/api/v1/packages/${name}`, {
                method: 'DELETE',
                credentials: 'same-origin',
                headers: {'X-Requested-With': 'XMLHttpRequest'},
            });
            const body = await response.json() as {packages?: unknown; message?: unknown};

            if (!response.ok) {
                setError(typeof body.message === 'string' ? body.message : 'The package could not be removed.');
                return;
            }
            if (Array.isArray(body.packages)) {
                setPackages(body.packages.filter((n): n is string => typeof n === 'string'));
            }
        } catch {
            setError('The package could not be removed.');
        } finally {
            if (live.current) {
                setBusy(null);
            }
        }
    }, []);

    return (
        <div style={styles.root}>
            {packages.length === 0 ? (
                <p style={styles.empty}>{'No detail areas are installed. The map draws its global basemap everywhere.'}</p>
            ) : (
                <ul style={styles.list}>
                    {packages.map((name) => (
                        <li
                            key={name}
                            style={styles.row}
                        >
                            <span style={styles.name}>{name}</span>

                            {/*
                              * Remove is offered only for an area in the package
                              * directory. A bundled one is inside the plugin and
                              * a release is what replaces it, so a button here
                              * could only ever report a failure.
                              */}
                            {removable.includes(name) ? (
                                <button
                                    type='button'
                                    className='btn btn-tertiary btn-sm'
                                    disabled={busy !== null}
                                    onClick={() => {
                                        remove(name).catch(() => {
                                            // remove never rejects.
                                        });
                                    }}
                                >{'Remove'}</button>
                            ) : (
                                <span style={styles.bundled}>{'ships with the plugin'}</span>
                            )}
                        </li>
                    ))}
                </ul>
            )}

            <input
                ref={file}
                type='file'
                accept='.pmtiles'
                disabled={busy !== null}
                onChange={(event) => {
                    const chosen = event.target.files?.[0];
                    event.target.value = '';
                    if (chosen) {
                        upload(chosen).catch(() => {
                            // upload never rejects.
                        });
                    }
                }}
            />

            {busy !== null && <p style={styles.busy}>{`Installing ${busy}…`}</p>}
            {error !== null && <p style={styles.error}>{error}</p>}

            <p style={styles.note}>
                {'A package is one area, named '}
                <code>{'<command>-<area>.pmtiles'}</code>
                {'. Uploading writes it into the package directory above, which is the same place a file copied by hand is read from. In a cluster an upload reaches only the node that served it, so shared storage or a copy per node is required either way. Areas larger than '}
                {MAX_UPLOAD_MB}
                {' MB should be copied into the directory rather than uploaded, because an upload crosses this server’s own request limits.'}
            </p>
        </div>
    );
};

export default PackageUploader;
