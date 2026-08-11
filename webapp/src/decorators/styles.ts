import {all, decoratePathPrefix} from './registry';

const STYLE_ELEMENT_ID = 'tactical-fusion-decorator-styles';

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
 */
export function buildDecoratorStyles(): string {
    const prefix = decoratePathPrefix();

    const rules = all().map((decorator) => {
        const selector = `a[href*="${cssEscape(prefix + decorator.type)}?"]`;
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

    return rules.join('\n\n');
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
