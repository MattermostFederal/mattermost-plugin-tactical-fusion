import React from 'react';

import CyberDatasetsHarness from './CyberDatasetsHarness';

import {expect, test} from '../../playwright/ct-coverage';

test.describe('the loaded datasets', () => {
    test('list each file with its records, size, date and source', async ({mount}) => {
        const view = await mount(<CyberDatasetsHarness/>);

        const table = view.getByTestId('cyber-datasets');
        const cve = table.getByRole('row').filter({hasText: 'Vulnerability'}).first();
        await expect(cve).toContainText('cve.tsv');
        await expect(cve).toContainText('your directory');
        await expect(cve).toContainText('396,474');
        await expect(cve).toContainText('177.1 MB');
        await expect(cve).toContainText('2026-09-23 20:57 UTC');

        const kev = table.getByRole('row').filter({hasText: 'Known exploited'});
        await expect(kev).toContainText('bundled');
        await expect(kev).toContainText('1,721');
        await expect(kev).toContainText('CISA KEV catalog 2026.09.23');
    });

    test('show vendor databases, what is missing, what is replaced and what was skipped', async ({mount}) => {
        const view = await mount(<CyberDatasetsHarness/>);

        await expect(view.getByText('dbip-city-lite.mmdb')).toBeVisible();
        await expect(view.getByText('built 2026-09-01')).toBeVisible();
        await expect(view.getByText('Not installed: watchlist (watchlist.tsv).')).toBeVisible();
        await expect(view.getByText('/plugins/tactical-fusion/assets/cyber/advisory.tsv')).toBeVisible();
        await expect(view.getByTestId('cyber-datasets-skipped')).toContainText('(TF-21003)');
    });

    test('say so when nothing is loaded', async ({mount}) => {
        const view = await mount(<CyberDatasetsHarness reply='empty'/>);

        await expect(view.getByText('No dataset is loaded.')).toBeVisible();
    });

    for (const reply of ['refused', 'not-a-list'] as const) {
        test(`say the list could not be read rather than showing an empty one (${reply})`, async ({mount}) => {
            const view = await mount(<CyberDatasetsHarness reply={reply}/>);

            await expect(view.getByText(/^The dataset list could not be read\./)).toBeVisible();
            await expect(view.getByText('No dataset is loaded.')).toHaveCount(0);
        });
    }

    test('mark a count that failed rather than showing zero', async ({mount}) => {
        const view = await mount(<CyberDatasetsHarness reply='count-failed'/>);

        const unknown = view.getByText('unknown', {exact: true});
        await expect(unknown).toBeVisible();
        await expect(unknown).toHaveAttribute('title', /TF-21001/);
    });
});
