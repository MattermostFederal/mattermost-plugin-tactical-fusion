import {parseDecoratorHref} from './registry';

const LABEL = /^[A-Za-z][A-Za-z0-9]{1,15}[ \t]*:[ \t]*/;

const SOLE_LINK = /^\[((?:\\.|[^\\[\]])*)\]\(([^()\s]+)\)$/;

export interface SoleLink {
    label: string;
    href: string;
}

export function soleLink(message: string): SoleLink | null {
    const match = SOLE_LINK.exec(message.trim().replace(LABEL, ''));
    if (match === null) {
        return null;
    }
    return {label: unescapeLabel(match[1]), href: match[2]};
}

export function soleLead(message: string): string {
    return LABEL.exec(message.trim())?.[0].trim() ?? '';
}

export interface SoleDecoratorLink {
    type: string;
    params: URLSearchParams;
    href: string;
    label: string;
    lead: string;
}

export function soleDecoratorLink(message: string): SoleDecoratorLink | null {
    const link = soleLink(message);
    if (link === null) {
        return null;
    }

    const parsed = parseDecoratorHref(link.href);
    if (parsed === null) {
        return null;
    }

    return {...parsed, href: link.href, label: link.label, lead: soleLead(message)};
}

function unescapeLabel(label: string): string {
    return label.replace(/\\(.)/g, '$1');
}

const ANY_LINK = /\[((?:\\.|[^\\[\]])*)\]\(([^()\s]+)\)/g;

export interface DecoratorLink {
    type: string;
    params: URLSearchParams;
    href: string;
    label: string;
    start: number;
    end: number;
}

export function decoratorLinks(message: string): DecoratorLink[] {
    const links: DecoratorLink[] = [];
    for (const match of message.matchAll(ANY_LINK)) {
        const parsed = parseDecoratorHref(match[2]);
        if (parsed === null) {
            continue;
        }
        links.push({
            ...parsed,
            href: match[2],
            label: unescapeLabel(match[1]),
            start: match.index,
            end: match.index + match[0].length,
        });
    }
    return links;
}
