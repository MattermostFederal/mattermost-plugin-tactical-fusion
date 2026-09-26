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

const imageHtml = '<p>before <img src="https://images.example.test/pixel.png" alt="pixel"> after</p>';

test('the hover card renders no image and keeps its alt text', async ({mount, page}) => {
    const fetched: string[] = [];
    page.on('request', (request) => {
        if (request.url().includes('images.example.test')) {
            fetched.push(request.url());
        }
    });
    await page.evaluate((html) => {
        window.PostUtils = {
            formatText: () => html,
            messageHtmlToComponent: (rendered: string) => `component:${rendered}`,
        };
    }, imageHtml);

    const hover = await mount(<NoteHover payload={{markdown: '![pixel](https://images.example.test/pixel.png)'}}/>);

    await expect(hover.getByTestId('note-markdown')).toHaveText('component:<p>before pixel after</p>');
    expect(fetched).toEqual([]);
});

test('the hover card drops every image the renderer wrote, reference style included', async ({mount, page}) => {
    await page.evaluate(() => {
        window.PostUtils = {
            formatText: () => '<p><img src="https://a.example.test/1.png" alt="one"><a href="https://b.example.test"><img src="https://b.example.test/2.png"></a></p>',
            messageHtmlToComponent: (rendered: string) => rendered,
        };
    });

    const hover = await mount(<NoteHover payload={{markdown: '![one][1]\n[![](https://b.example.test/2.png)](https://b.example.test)\n\n[1]: https://a.example.test/1.png'}}/>);

    await expect(hover.getByTestId('note-markdown')).not.toContainText('<img');
    await expect(hover.getByTestId('note-markdown')).toContainText('one');
});

test('the panel keeps the images the renderer wrote', async ({mount, page}) => {
    await page.evaluate((html) => {
        window.PostUtils = {
            formatText: () => html,
            messageHtmlToComponent: (rendered: string) => `component:${rendered}`,
        };
    }, imageHtml);

    const panel = await mount(<NotePanel payload={{markdown: '![pixel](https://images.example.test/pixel.png)'}}/>);

    await expect(panel.getByTestId('note-markdown')).toHaveText(`component:${imageHtml}`);
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
