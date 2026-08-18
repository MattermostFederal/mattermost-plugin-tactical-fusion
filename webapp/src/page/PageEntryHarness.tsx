import React, {useEffect, useState} from 'react';

/**
 * Harness for the page bundle's entry point.
 *
 * `src/page/index.tsx` exports nothing and does its work at import time, so the
 * only way to exercise it is to import it in a browser with the document
 * already arranged the way the server's shell arranges it. `IndexHarness` does
 * the same thing for the webapp entry point, and for the same reason.
 *
 * The one complication is that Playwright's own mount container is also
 * `#root`, which is the element the entry point looks for. So the harness lends
 * the id to a div of its own for the duration of the import and hands it back
 * afterwards: the entry point keeps the element reference it captured, and
 * Playwright can still find its container to unmount. Assertions therefore go
 * through `#page-root` rather than through the mounted component's locator, and
 * they must wait for `entry-state` first, since none of it exists until the
 * import settles.
 *
 * The hand-back is in a `finally` and renames the planted div in both
 * directions. It was written twice, and the failure branch renamed nothing, so
 * an import that threw left two elements carrying `id="root"` on exactly the
 * path where something had already gone wrong.
 *
 * The planted div and the React root inside it are deliberately NOT cleaned up.
 * The entry point cannot be unmounted or re-imported, each component test gets
 * its own page, and the next test navigates before anything else runs.
 */

export interface Plant {
    f?: string;
    v?: string;
    r?: string;
    conversion?: string;
    mode?: string;
}

interface Props {

    /** The shell the server would have written, or null for a page without one. */
    plant?: Plant | null;
}

/*
 * An ES module is evaluated once per page, so a second run of the effect could
 * not re-exercise the entry point and could only plant a second `#root`. The
 * flag states that in code rather than only in the comment above.
 */
let started = false;

const PageEntryHarness: React.FC<Props> = ({plant = null}) => {
    const [state, setState] = useState('importing');

    useEffect(() => {
        if (started) {
            return;
        }
        started = true;

        const borrowed = document.getElementById('root');
        borrowed?.removeAttribute('id');

        let planted: HTMLDivElement | null = null;
        if (plant) {
            planted = document.createElement('div');
            planted.id = 'root';
            for (const [key, value] of Object.entries(plant)) {
                // Object.entries keeps a key set to undefined, and dataset would
                // write the string "undefined", which readPageData reads as a
                // value rather than as an absent attribute.
                if (value !== undefined) {
                    planted.dataset[key] = value;
                }
            }
            document.body.appendChild(planted);
        }

        const handBack = () => {
            if (planted) {
                planted.id = 'page-root';
            }
            if (borrowed) {
                borrowed.id = 'root';
            }
        };

        // The hand-back completes BEFORE the state flips, so a test that waits
        // on `entry-state` and then reads `#page-root` cannot observe the
        // window in between.
        import('./index').
            then(() => 'imported', (err: unknown) => `threw: ${String(err)}`).
            then((next) => {
                handBack();
                setState(next);
            });
    }, [plant]);

    return <output data-testid='entry-state'>{state}</output>;
};

export default PageEntryHarness;
