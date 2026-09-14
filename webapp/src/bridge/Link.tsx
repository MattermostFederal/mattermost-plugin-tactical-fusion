import React, {useEffect, useState} from 'react';

import {link} from './client';
import type {DecoratorLink, LinkProps} from './types';

import HoverLink from '../decorators/HoverLink';

type LinkState =
    | {status: 'loading'}
    | {status: 'linked'; link: DecoratorLink}
    | {status: 'declined'};

export const Link: React.FC<LinkProps> = ({type, token, label, referenceTime, fallback}) => {
    const [state, setState] = useState<LinkState>({status: 'loading'});
    const reference = referenceTime instanceof Date ? referenceTime.getTime() : referenceTime;

    useEffect(() => {
        let mounted = true;
        setState({status: 'loading'});

        link(type, token, {label, referenceTime: reference}).then(
            (result) => {
                if (mounted) {
                    setState({status: 'linked', link: result});
                }
            },
            () => {
                if (mounted) {
                    setState({status: 'declined'});
                }
            },
        );

        return () => {
            mounted = false;
        };
    }, [type, token, label, reference]);

    if (state.status === 'linked') {
        return <HoverLink href={state.link.url}>{state.link.label}</HoverLink>;
    }

    if (state.status === 'declined' && fallback !== undefined) {
        return <>{fallback}</>;
    }

    return <span data-tactical-fusion-link={state.status}>{label || token}</span>;
};

export default Link;
