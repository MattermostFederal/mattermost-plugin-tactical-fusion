import {useEffect, useState} from 'react';

export const NEAR_VIEWPORT_MARGIN = '300px 0px';

export const RELEASE_AFTER_MS = 2000;

export function useNearViewport(
    element: Element | null,
    margin: string = NEAR_VIEWPORT_MARGIN,
    releaseAfterMs: number = RELEASE_AFTER_MS,
): boolean {
    const [near, setNear] = useState(typeof IntersectionObserver === 'undefined');

    useEffect(() => {
        if (typeof IntersectionObserver === 'undefined' || element === null) {
            return undefined;
        }

        let release: ReturnType<typeof setTimeout> | undefined;
        const cancelRelease = () => {
            if (release !== undefined) {
                clearTimeout(release);
                release = undefined;
            }
        };

        const observer = new IntersectionObserver((entries) => {
            const visible = entries.some((entry) => entry.isIntersecting);
            if (visible) {
                cancelRelease();
                setNear(true);
                return;
            }
            cancelRelease();
            release = setTimeout(() => setNear(false), releaseAfterMs);
        }, {rootMargin: margin});

        observer.observe(element);

        return () => {
            cancelRelease();
            observer.disconnect();
        };
    }, [element, margin, releaseAfterMs]);

    return near;
}
