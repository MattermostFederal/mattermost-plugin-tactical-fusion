import {HOVER_CARD_CLASS, all, decoratePathPrefix} from './registry';

const STYLE_ELEMENT_ID = 'tactical-fusion-decorator-styles';

/**
 * Hides the hover card's chrome when the decorator's Hover renders nothing.
 *
 * A Hover that returns null leaves the card's own padding, border and shadow
 * floating beside the link with nothing inside them, because the framework has
 * already built that box by the time the component decides. Nothing inline can
 * express this: `:empty` is a selector, and every style in this plugin bar this
 * sheet is an inline style attribute, which has no place to put one.
 *
 * That distinction is what lets a decorator answer "no card right now" at render
 * rather than only by declining to declare a Hover at all, which is what the
 * location card needs when an admin has turned maps off.
 */
const EMPTY_HOVER_RULE = [
    `.${HOVER_CARD_CLASS}:empty {`,
    '    display: none;',
    '}',
].join('\n');

/** Escapes a value for use inside a CSS attribute selector string. */
function cssEscape(value: string): string {
    return value.replace(/["\\]/g, '\\$&');
}

/**
 * Builds the stylesheet, one rule per registered decorator.
 *
 * Selectors key on the full plugin path rather than the bare `/decorate/<type>`
 * suffix, so an unrelated link elsewhere in the page cannot match. The prefix
 * comes from the same helper the click handler uses, so the two cannot drift.
 *
 * Matched from the start of the href, not anywhere inside it. The server only
 * ever writes root-relative destinations, so every real decorator link begins
 * with this prefix, while a substring match also styled anybody's absolute URL
 * that merely carried the path in its query string. That handed an arbitrary
 * posted link the plugin's own chip, which is a claim this stylesheet has no
 * business making about a destination it does not own.
 */
export function buildDecoratorStyles(): string {
    const prefix = decoratePathPrefix();

    const rules = all().map((decorator) => {
        const selector = `a[href^="${cssEscape(prefix + decorator.type)}?"]`;
        return [
            `${selector} {`,
            '    text-decoration: none;',
            '    border-radius: 3px;',
            '    padding: 1px 4px;',
            '    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;',
            '    font-size: 0.95em;',
            `    color: ${decorator.style.color};`,
            `    background: ${decorator.style.background};`,
            '}',
            `${selector}:hover {`,
            '    text-decoration: underline;',
            '}',
        ].join('\n');
    });

    return [...rules, EMPTY_HOVER_RULE].join('\n\n');
}

/**
 * Appends the stylesheet to the document head.
 *
 * A plain function rather than a React component: nothing here is dynamic, and
 * this avoids registerGlobalComponent, which the webapp type definitions flag as
 * internal and subject to change without notice.
 *
 * Idempotent, and returns a disposer.
 */
export function installDecoratorStyles(): () => void {
    const existing = document.getElementById(STYLE_ELEMENT_ID);
    if (existing) {
        return () => {
            // Already installed by an earlier call, which owns the element.
        };
    }

    const style = document.createElement('style');
    style.id = STYLE_ELEMENT_ID;
    style.textContent = buildDecoratorStyles();
    document.head.appendChild(style);

    return () => {
        style.remove();
    };
}
