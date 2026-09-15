import React, {useEffect, useState} from 'react';

import {link} from './client';
import type {DecoratorLink, LinkProps} from './types';

import HoverLink from '../decorators/HoverLink';

type SettledLink =
    | {request: string; status: 'linked'; link: DecoratorLink}
    | {request: string; status: 'declined'};

export const Link: React.FC<LinkProps> = ({type, token, label, referenceTime, fallback}) => {
    const [settled, setSettled] = useState<SettledLink | null>(null);
    const reference = referenceTime instanceof Date ? referenceTime.getTime() : referenceTime;
    const request = JSON.stringify([type, token, label ?? '', reference ?? null]);

    useEffect(() => {
        let mounted = true;

        link(type, token, {label, referenceTime: reference}).then(
            (result) => {
                if (mounted) {
                    setSettled({request, status: 'linked', link: result});
                }
            },
            () => {
                if (mounted) {
                    setSettled({request, status: 'declined'});
                }
            },
        );

        return () => {
            mounted = false;
        };
    }, [request]); // eslint-disable-line react-hooks/exhaustive-deps

    const current = settled?.request === request ? settled : null;

    if (current?.status === 'linked') {
        return <HoverLink href={current.link.url}>{current.link.label}</HoverLink>;
    }

    if (current?.status === 'declined' && fallback !== undefined) {
        return <>{fallback}</>;
    }

    return <span data-tactical-fusion-link={current?.status ?? 'loading'}>{label || token}</span>;
};

export default Link;
