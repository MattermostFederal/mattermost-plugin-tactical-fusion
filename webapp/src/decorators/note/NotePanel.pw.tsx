import React from 'react';

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
