import React from 'react';

import {HONOLULU_METAR} from './report_fixtures';
import ReportLinkPanelHarness from './ReportLinkPanelHarness';
import {SOURCE_LABEL, STATUS_TEXT} from './ReportPanel';

import {expect, test} from '../../playwright/ct-coverage';

test.describe('the link panel', () => {
    test('decodes the report behind the link', async ({mount}) => {
        const panel = await mount(
            <ReportLinkPanelHarness
                surface='panel'
                reply='found'
            />);

        await expect(panel.getByTestId('avreport-heading')).toHaveCount(0);
        await expect(panel.getByTestId('avreport-summary')).toHaveCount(0);
        await expect(panel.getByTestId('avreport-rows')).toContainText('30.10 inHg');
        await expect(panel.getByRole('button', {name: 'Daniel K. Inouye International Airport'})).toBeVisible();
    });

    test('keeps the report as posted collapsed under the map', async ({mount}) => {
        const panel = await mount(
            <ReportLinkPanelHarness
                surface='panel'
                reply='found'
                maps={true}
            />);

        await expect(panel.getByTestId('avreport-source')).toBeHidden();
        await panel.getByText(SOURCE_LABEL).click();
        await expect(panel.getByTestId('avreport-source')).toHaveText(HONOLULU_METAR.src);
        await expect(panel.getByRole('button', {name: 'Copy the report as posted'})).toBeVisible();

        const order = await panel.locator('[data-testid="avreport-rows"], [data-testid="avreport-source"], [data-testid="avreport-map"]').evaluateAll(
            (nodes) => nodes.map((node) => node.getAttribute('data-testid')),
        );
        expect(order).toEqual(['avreport-rows', 'avreport-map', 'avreport-source']);
    });

    test('a station outside the database gets no name line', async ({mount}) => {
        const panel = await mount(
            <ReportLinkPanelHarness
                surface='panel'
                reply='unplaced'
            />);

        await expect(panel.getByTestId('avreport-rows')).toBeVisible();
        await expect(panel.getByRole('button', {name: 'Daniel K. Inouye International Airport'})).toBeHidden();
    });

    for (const reply of ['hold', 'failed', 'rejected'] as const) {
        test(`says so while ${reply}, and keeps the report on screen`, async ({mount}) => {
            const panel = await mount(
                <ReportLinkPanelHarness
                    surface='panel'
                    reply={reply}
                />);

            const status = reply === 'hold' ? STATUS_TEXT.loading : STATUS_TEXT[reply];
            await expect(panel.getByTestId('avreport-status')).toHaveText(status);
            await expect(panel.getByTestId('avreport-source')).toHaveText(HONOLULU_METAR.src);
            await expect(panel.getByTestId('avreport-rows')).toBeHidden();
        });
    }
});

test.describe('the hover', () => {
    test('names the station and gives the one-line summary', async ({mount}) => {
        const hover = await mount(
            <ReportLinkPanelHarness
                surface='hover'
                reply='found'
            />);

        await expect(hover.getByText('METAR PHNL, Daniel K. Inouye International Airport')).toBeVisible();
        await expect(hover.getByText(/Wind 070°/)).toBeVisible();
    });

    for (const reply of ['hold', 'failed', 'rejected'] as const) {
        test(`renders nothing while ${reply}`, async ({mount}) => {
            const hover = await mount(
                <ReportLinkPanelHarness
                    surface='hover'
                    reply={reply}
                />);

            await expect(hover.getByTestId('surface')).toBeAttached();
            await expect(hover.getByTestId('surface')).toBeEmpty();
        });
    }
});

test.describe('the title', () => {
    test('is the kind and the station once decoded', async ({mount}) => {
        const title = await mount(
            <ReportLinkPanelHarness
                surface='title'
                reply='found'
            />);

        await expect(title.getByTestId('surface')).toHaveText('METAR PHNL');
    });

    for (const reply of ['hold', 'failed', 'rejected'] as const) {
        test(`reads the kind and station off the link while ${reply}`, async ({mount}) => {
            const title = await mount(
                <ReportLinkPanelHarness
                    surface='title'
                    reply={reply}
                />);

            await expect(title.getByTestId('surface')).toHaveText('METAR PHNL');
        });
    }
});
