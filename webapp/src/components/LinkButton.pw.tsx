import React from 'react';

import LinkButtonHarness from './LinkButtonHarness';

import {expect, test} from '../../playwright/ct-coverage';

// DtgPanel.pw.tsx already covers the two shapes in place and the underline
// appearing on hover and on focus. What is left is the other half of each of
// those handlers, the props that only one of the two branches honors, and the
// style precedence the doc comment describes.

test.describe('which element it renders', () => {
    test('renders a button by default', async ({mount, page}) => {
        await mount(<LinkButtonHarness/>);

        await expect(page.getByRole('button', {name: 'Customize your view'})).toBeVisible();
        await expect(page.locator('a')).toHaveCount(0);
    });

    test('renders an anchor that opens a new tab when given an href', async ({mount, page}) => {
        await mount(<LinkButtonHarness href='/plugins/x/public/help/help.html'/>);

        const link = page.getByRole('link', {name: 'Customize your view'});
        await expect(link).toHaveAttribute('href', '/plugins/x/public/help/help.html');
        await expect(link).toHaveAttribute('target', '_blank');
        await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    // The guard is `href !== undefined`, not a truthiness check, so an empty
    // string still picks the anchor. Pinned because the two branches differ in
    // what they honor.
    test('an empty href still renders an anchor', async ({mount, page}) => {
        await mount(<LinkButtonHarness href=''/>);

        await expect(page.locator('a')).toHaveCount(1);
        await expect(page.locator('button', {hasText: 'Customize your view'})).toHaveCount(0);
    });
});

test.describe('the underline', () => {
    test('appears on hover and goes again when the pointer leaves', async ({mount, page}) => {
        await mount(<LinkButtonHarness/>);

        const button = page.getByRole('button', {name: 'Customize your view'});
        await expect(button).toHaveCSS('text-decoration-line', 'none');

        await button.hover();
        await expect(button).toHaveCSS('text-decoration-line', 'underline');

        await page.getByTestId('elsewhere').hover();
        await expect(button).toHaveCSS('text-decoration-line', 'none');
    });

    // Focus counts as well as hover, or the cue would be invisible to anybody
    // moving through the panel by keyboard.
    test('appears on focus and goes again on blur', async ({mount, page}) => {
        await mount(<LinkButtonHarness/>);

        const button = page.getByRole('button', {name: 'Customize your view'});

        await button.focus();
        await expect(button).toHaveCSS('text-decoration-line', 'underline');

        await page.getByTestId('elsewhere').focus();
        await expect(button).toHaveCSS('text-decoration-line', 'none');
    });

    test('the anchor underlines the same way', async ({mount, page}) => {
        await mount(<LinkButtonHarness href='/somewhere'/>);

        const link = page.getByRole('link', {name: 'Customize your view'});
        await expect(link).toHaveCSS('text-decoration-line', 'none');

        await link.hover();
        await expect(link).toHaveCSS('text-decoration-line', 'underline');

        await page.getByTestId('elsewhere').hover();
        await expect(link).toHaveCSS('text-decoration-line', 'none');
    });
});

test.describe('clicking', () => {
    test('calls onClick', async ({mount, page}) => {
        await mount(<LinkButtonHarness/>);

        await page.getByRole('button', {name: 'Customize your view'}).click();

        await expect(page.getByTestId('clicks')).toHaveText('1');
    });

    test('a button with no onClick is harmless', async ({mount, page}) => {
        await mount(<LinkButtonHarness withOnClick={false}/>);

        await page.getByRole('button', {name: 'Customize your view'}).click();

        await expect(page.getByTestId('clicks')).toHaveText('0');
    });

    // Documented: "Ignored when `href` is set." The anchor never receives the
    // handler, so following the link is the only thing it does.
    test('ignores onClick when an href is set', async ({mount, page}) => {
        await mount(<LinkButtonHarness href='/somewhere'/>);

        await page.getByRole('link', {name: 'Customize your view'}).click();

        await expect(page.getByTestId('clicks')).toHaveText('0');
    });
});

test.describe('disabled', () => {
    test('a disabled button is inert', async ({mount, page}) => {
        await mount(<LinkButtonHarness disabled={true}/>);

        const button = page.getByRole('button', {name: 'Customize your view'});
        await expect(button).toBeDisabled();

        await button.click({force: true});

        await expect(page.getByTestId('clicks')).toHaveText('0');
    });

    test('a disabled button does not underline on hover', async ({mount, page}) => {
        await mount(<LinkButtonHarness disabled={true}/>);

        const button = page.getByRole('button', {name: 'Customize your view'});
        await button.hover({force: true});

        await expect(button).toHaveCSS('text-decoration-line', 'none');
    });

    // `disabled` is documented "Buttons only" and is spread onto the button
    // alone, so an anchor asked to be disabled is not. Pinned so the asymmetry
    // is a decision rather than a surprise: a caller wanting an inert link has
    // to withhold the href.
    test('is not honored on the anchor branch', async ({mount, page}) => {
        await mount(<LinkButtonHarness
            href='/somewhere'
            disabled={true}
                    />);

        const link = page.getByRole('link', {name: 'Customize your view'});
        await expect(link).not.toHaveAttribute('disabled', /.*/);
        await expect(link).toHaveAttribute('href', '/somewhere');
    });
});

test.describe('style', () => {
    test('placement and type size come from the caller', async ({mount, page}) => {
        await mount(<LinkButtonHarness style={{fontSize: '12px', margin: '8px'}}/>);

        const button = page.getByRole('button', {name: 'Customize your view'});
        await expect(button).toHaveCSS('font-size', '12px');
        await expect(button).toHaveCSS('margin-top', '8px');
    });

    // The underline is applied after the caller's style, so it always wins.
    // That is what keeps hover and focus working whatever is passed in.
    test('a caller cannot pin the text decoration', async ({mount, page}) => {
        await mount(<LinkButtonHarness style={{textDecoration: 'underline'}}/>);

        const button = page.getByRole('button', {name: 'Customize your view'});
        await expect(button).toHaveCSS('text-decoration-line', 'none');

        await button.hover();
        await expect(button).toHaveCSS('text-decoration-line', 'underline');
    });

    // The doc comment on `style` says "The link coloring is not overridable",
    // but the caller's style is spread after the base, so a color in it does
    // win. Pinned as it behaves; the comment is what is wrong.
    test('a caller-supplied color does override the link color', async ({mount, page}) => {
        await mount(<LinkButtonHarness style={{color: 'rgb(1, 2, 3)'}}/>);

        await expect(page.getByRole('button', {name: 'Customize your view'})).
            toHaveCSS('color', 'rgb(1, 2, 3)');
    });
});
