export type Theme = 'light' | 'dark';

/**
 * Classifies a CSS color as a light or dark background.
 *
 * Returns null when the color cannot be read, so callers can leave the theme
 * unstated rather than guess. Accepts the forms Mattermost actually sets its
 * theme variables to: `#rgb`, `#rrggbb`, and `rgb()`/`rgba()`.
 */
export function themeForBackground(color: string): Theme | null {
    const rgb = parseColor(color.trim());
    if (!rgb) {
        return null;
    }

    // Rec. 601 luma. Precision beyond "which side of the middle" is not needed,
    // and this is cheap and stable across the colors a theme actually uses.
    const [r, g, b] = rgb;
    const luma = ((0.299 * r) + (0.587 * g) + (0.114 * b)) / 255;

    return luma < 0.5 ? 'dark' : 'light';
}

function parseColor(color: string): [number, number, number] | null {
    const hex = (/^#([0-9a-f]{3}|[0-9a-f]{6})$/i).exec(color);
    if (hex) {
        const digits = hex[1];
        const expanded = digits.length === 3 ? digits.split('').map((d) => d + d).join('') : digits;
        return [
            parseInt(expanded.slice(0, 2), 16),
            parseInt(expanded.slice(2, 4), 16),
            parseInt(expanded.slice(4, 6), 16),
        ];
    }

    const rgb = (/^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)/i).exec(color);
    if (rgb) {
        return [Number(rgb[1]), Number(rgb[2]), Number(rgb[3])];
    }

    return null;
}

/**
 * The theme the reader currently has in Mattermost.
 *
 * Read from the live CSS variable rather than the Redux store, so it reflects
 * exactly what the sidebar is painted with, needs no `mattermost-redux`
 * dependency, and keeps working if the theme is changed without a reload.
 *
 * Returns null when the variable is unavailable, in which case the theme is
 * left unstated and the page falls back to the operating system preference.
 */
export function detectTheme(): Theme | null {
    if (typeof window === 'undefined' || !window.getComputedStyle) {
        return null;
    }

    for (const element of [document.documentElement, document.body]) {
        if (!element) {
            continue;
        }
        const value = window.getComputedStyle(element).getPropertyValue('--center-channel-bg');
        const theme = value ? themeForBackground(value) : null;
        if (theme) {
            return theme;
        }
    }

    return null;
}
