import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {HOVER_CARD_CLASS, register, _resetForTesting} from './registry';
import {buildDecoratorStyles} from './styles';
import type {Decorator} from './types';

function fixture(type: string, color: string): Decorator<unknown> {
    return {
        type,
        fromParams: () => ({}),
        summary: () => type,
        style: {color, background: 'transparent'},
        Panel: (() => null) as unknown as Decorator<unknown>['Panel'],
    };
}

test.beforeEach(() => {
    _resetForTesting();
});

test('emits one rule per registered decorator', () => {
    register(fixture('alpha', '#111111'));
    register(fixture('beta', '#222222'));

    const css = buildDecoratorStyles();

    expect(css).toContain('#111111');
    expect(css).toContain('#222222');
    expect(css).toContain(`/plugins/${manifest.id}/decorate/alpha?`);
    expect(css).toContain(`/plugins/${manifest.id}/decorate/beta?`);
});

// Keying on the bare "/decorate/<type>" suffix would let an unrelated link
// elsewhere in the page pick up decorator styling.
test('selectors carry the full plugin path', () => {
    register(fixture('alpha', '#111111'));

    expect(buildDecoratorStyles()).toContain(`a[href^="/plugins/${manifest.id}/decorate/alpha?"]`);
});

// Matched from the start of the href, not anywhere inside it. A substring match
// also styled an absolute URL that merely carried the path in its query string,
// which handed an arbitrary posted link this plugin's own chip.
test('selectors match from the start of the href', () => {
    register(fixture('alpha', '#111111'));

    const css = buildDecoratorStyles();
    expect(css).toContain('a[href^=');
    expect(css).not.toContain('a[href*=');
});

// The trailing "?" stops "dtg" from also matching a future "dtg2".
test('selectors are anchored with a trailing question mark', () => {
    register(fixture('dtg', '#111111'));

    expect(buildDecoratorStyles()).toContain('decorate/dtg?"]');
});

// No decorators means no per-decorator chip rules. The sheet is not empty even
// then, because the hover-card rule below is the framework's own rather than any
// decorator's, and a decorator registered later needs it already in place.
test('an empty registry produces no per-decorator rules', () => {
    const css = buildDecoratorStyles();

    expect(css).not.toContain('/decorate/');
    expect(css).toBe(`.${HOVER_CARD_CLASS}:empty {\n    display: none;\n}`);
});

/*
 * The rule that lets a decorator decline a card at render rather than only by
 * declaring no Hover at all.
 *
 * It cannot be an inline style: `:empty` is a selector, and every other style in
 * this plugin is an inline style attribute with nowhere to put one. So the sheet
 * is the only place it can live, and the class it names has to be the one
 * Tooltip.tsx puts on the element.
 */
test('hides a hover card that renders nothing', () => {
    register(fixture('alpha', '#111111'));

    const css = buildDecoratorStyles();

    expect(css).toContain(`.${HOVER_CARD_CLASS}:empty`);
    expect(css).toContain('display: none;');
});
