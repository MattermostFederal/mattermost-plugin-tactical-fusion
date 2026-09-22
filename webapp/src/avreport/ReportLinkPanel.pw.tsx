import React from 'react';

import {HONOLULU_METAR} from './report_fixtures';
import ReportLinkPanelHarness from './ReportLinkPanelHarness';
import {STATUS_TEXT} from './ReportPanel';

import {expect, test} from '../../playwright/ct-coverage';

test.describe('the link panel', () => {
    test('decodes the report behind the link', async ({mount}) => {
        const panel = await mount(
            <ReportLinkPanelHarness
                surface='panel'
                reply='found'
            />);

        await expect(panel.getByTestId('avreport-heading')).toHaveText('METAR PHNL');
        await expect(panel.getByTestId('avreport-summary')).toContainText('Wind 070°');
        await expect(panel.getByTestId('avreport-source')).toHaveText(HONOLULU_METAR.src);
        await expect(panel.getByTestId('avreport-rows')).toContainText('30.10 inHg');
        await expect(panel.getByRole('button', {name: 'Daniel K. Inouye International Airport'})).toBeVisible();
    });

    test('a station outside the database gets no name line', async ({mount}) => {
        const panel = await mount(
            <ReportLinkPanelHarness
                surface='panel'
                reply='unplaced'
            />);

        await expect(panel.getByTestId('avreport-heading')).toHaveText('METAR PHNL');
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
            await expect(panel.getByTestId('avreport-heading')).toBeHidden();
        });
    }
});

test.describe('the hover', () => {
    test('is the summary line', async ({mount}) => {
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
