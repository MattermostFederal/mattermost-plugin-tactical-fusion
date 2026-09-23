import React from 'react';

import NoteHarness from './NoteHarness';
import NoteHover from './NoteHover';
import NotePanel from './NotePanel';

import {expect, test} from '../../../playwright/ct-coverage';

const markdown = '**DCA**: Defensive Counter Air\n\n<script>alert(1)</script>';

test('renders through the Mattermost markdown renderer when it is there', async ({mount, page}) => {
    await page.evaluate(() => {
        window.PostUtils = {
            formatText: (text: string) => `formatted:${text}`,
            messageHtmlToComponent: (html: string) => `component:${html}`,
        };
    });

    const hover = await mount(<NoteHover payload={{markdown}}/>);

    await expect(hover.getByTestId('note-markdown')).toContainText('component:formatted:**DCA**');
    await expect(hover.getByTestId('note-source-fallback')).toBeHidden();
});

test('proxies images exactly when the server has an image proxy', async ({mount, page}) => {
    await page.evaluate(() => {
        window.PostUtils = {
            formatText: (_text: string, options?: {proxyImages?: boolean}) => `proxyImages=${String(options?.proxyImages)}`,
            messageHtmlToComponent: (html: string) => html,
        };
    });

    const proxied = await mount(
        <NoteHarness
            markdown='![](https://example.com/p.png)'
            imageProxy={true}
        />,
    );
    await expect(proxied.getByTestId('note-markdown')).toHaveText('proxyImages=true');
    await proxied.unmount();

    const direct = await mount(
        <NoteHarness
            markdown='![](https://example.com/p.png)'
            imageProxy={false}
        />,
    );
    await expect(direct.getByTestId('note-markdown')).toHaveText('proxyImages=false');
});

test('shows the source when the renderer returns nothing', async ({mount, page}) => {
    await page.evaluate(() => {
        window.PostUtils = {
            formatText: (text: string) => text,
            messageHtmlToComponent: () => undefined,
        };
    });

    const hover = await mount(<NoteHover payload={{markdown}}/>);

    await expect(hover.getByTestId('note-source-fallback')).toHaveText(markdown);
});

test('shows the source when the renderer is missing', async ({mount, page}) => {
    await page.evaluate(() => {
        delete window.PostUtils;
    });

    const hover = await mount(<NoteHover payload={{markdown}}/>);

    await expect(hover.getByTestId('note-source-fallback')).toHaveText(markdown);
});

test('shows the source when the renderer throws', async ({mount, page}) => {
    await page.evaluate(() => {
        window.PostUtils = {
            formatText: () => {
                throw new Error('no emoji map');
            },
            messageHtmlToComponent: () => null,
        };
    });

    const hover = await mount(<NoteHover payload={{markdown}}/>);

    await expect(hover.getByTestId('note-source-fallback')).toHaveText(markdown);
});

test('the panel keeps the markdown as posted behind a disclosure', async ({mount, page}) => {
    await page.evaluate(() => {
        delete window.PostUtils;
    });

    const panel = await mount(<NotePanel payload={{markdown}}/>);

    await expect(panel.getByTestId('note-source')).toBeHidden();
    await panel.getByRole('button', {name: 'As posted'}).click();
    await expect(panel.getByTestId('note-source')).toHaveText(markdown);
    await expect(panel.getByRole('button', {name: 'Copy the note as posted'})).toBeVisible();
});
