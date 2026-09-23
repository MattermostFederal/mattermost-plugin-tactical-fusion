import NoteHover from './NoteHover';
import NotePanel from './NotePanel';

import {HOVER_MAX_WIDTH} from '../Tooltip';
import type {Decorator} from '../types';

export const PANEL_TITLE = 'Note';

export const MAX_NOTE_RUNES = 1000;

export const NOTE_HOVER_MAX_WIDTH = Math.round(HOVER_MAX_WIDTH * 1.2);

export interface NotePayload {
    markdown: string;
}

function runeCount(text: string): number {
    return Array.from(text).length;
}

export function fromParams(params: URLSearchParams): NotePayload | null {
    const markdown = params.get('v');
    if (markdown === null || markdown.trim() === '' || runeCount(markdown) > MAX_NOTE_RUNES) {
        return null;
    }
    return {markdown};
}

const MARKDOWN_MARKERS = /^(?:#{1,6}\s+|>\s*|[-*+]\s+(?:\[[ xX]\]\s+)?|\d+\.\s+|\|)|[*_~`|]/g;

export function firstLine(markdown: string): string {
    for (const line of markdown.split('\n')) {
        if ((/^\s*\|?\s*:?-{2,}/).test(line)) {
            continue;
        }
        const text = line.replace(MARKDOWN_MARKERS, '').replace(/\s+/g, ' ').trim();
        if (text !== '') {
            return text;
        }
    }
    return '';
}

export const NOTE_COLOR = '#1b7a6e';

const decorator: Decorator<NotePayload> = {
    type: 'note',
    fromParams,

    summary: (payload) => {
        const line = firstLine(payload.markdown);
        return line === '' ? PANEL_TITLE : `${PANEL_TITLE}: ${line}`;
    },

    style: {color: NOTE_COLOR, background: 'rgba(27, 122, 110, 0.12)'},

    Panel: NotePanel,

    Hover: NoteHover,

    hoverMaxWidth: NOTE_HOVER_MAX_WIDTH,
};

export default decorator;
