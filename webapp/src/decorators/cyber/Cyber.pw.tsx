import React from 'react';

import CyberHarness from './CyberHarness';

import {expect, test} from '../../../playwright/ct-coverage';

const HEADLINE = '10.0 Critical, in KEV';
const SUMMARY = 'Remote code execution in a logging library.';

const CVE = {kind: 'cve', value: 'CVE-2021-44228'};
const CWE = {kind: 'cwe', value: 'CWE-79'};

test.describe('the panel', () => {
    test('shows the indicator and what is known about it', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText('CVE-2021-44228').first()).toBeVisible();
        await expect(panel.getByText(SUMMARY)).toBeVisible();
        await expect(panel.getByText('10.0 Critical')).toBeVisible();
        await expect(panel.getByText('2021-12-10')).toBeVisible();
    });

    test('leaves the dataset list to the standalone page', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText(SUMMARY)).toBeVisible();
        await expect(panel.getByText('Datasets', {exact: true})).toHaveCount(0);
        await expect(panel.getByText(/not installed/)).toHaveCount(0);
    });

    test('says so when no dataset holds the indicator', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='status'
            />,
        );

        await expect(panel.getByText('No vulnerability dataset is installed.').first()).toBeVisible();
    });

    test('shows the watchlist verdict an operator wrote', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText('malicious')).toBeVisible();
        await expect(panel.getByText('seen beaconing')).toBeVisible();
    });

    test('a related entity hands the sidebar the next indicator', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await panel.getByRole('button', {name: 'CWE-502 Deserialization of Untrusted Data'}).click();

        await expect(panel.getByTestId('selection')).toHaveText(
            'cyber:{"kind":"cwe","value":"CWE-502"}',
        );
    });

    test('says it is still looking while the request is in flight', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='hold'
            />,
        );

        await expect(panel.getByText('Looking this indicator up...')).toBeVisible();
    });

    test('says the server refused a link it did not issue', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='rejected'
            />,
        );

        await expect(panel.getByText('That is not an indicator this plugin issued.')).toBeVisible();
    });

    test('says the lookup failed rather than showing nothing', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='failed'
            />,
        );

        await expect(panel.getByText('This indicator could not be looked up. The server did not answer.')).toBeVisible();
    });

    test('follows a change of selection without remounting', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                second={CWE}
            />,
        );

        await expect(panel.getByText('CVE-2021-44228').first()).toBeVisible();

        await panel.getByRole('button', {name: 'Show the second'}).click();

        await expect(panel.getByText('CWE-79').first()).toBeVisible();
        await expect(panel.getByTestId('requests')).toHaveText('2');
    });
});

test.describe('the detail sections', () => {
    test('are collapsed, each with a count', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText('Affected, as reported (1)')).toBeVisible();
        await expect(panel.getByText('Affected, per NVD (2)')).toBeVisible();
        await expect(panel.getByText('References (1)')).toBeVisible();
        await expect(panel.getByText('apache log4j: from 2.0 before 2.3.1, from 2.4 before 2.12.2')).toBeHidden();
    });

    test('open to show every line', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await panel.getByText('Affected, per NVD (2)').click();

        await expect(panel.getByText('apache log4j: from 2.0 before 2.3.1, from 2.4 before 2.12.2')).toBeVisible();
        await expect(panel.getByText('siemens sppa-t3000 firmware: all versions (on siemens sppa-t3000)')).toBeVisible();
        await expect(panel.getByText('Apache Software Foundation Apache Log4j2: from 2.0-beta9 before 2.15.0')).toBeHidden();
    });

    test('open a reference in a new tab, with its tags beside it', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await panel.getByText('References (1)').click();

        const link = panel.getByRole('link', {name: 'https://logging.apache.org/log4j/2.x/security.html'});
        await expect(link).toHaveAttribute('href', 'https://logging.apache.org/log4j/2.x/security.html');
        await expect(link).toHaveAttribute('target', '_blank');
        await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
        await expect(panel.getByText('Vendor Advisory, Patch')).toBeVisible();
    });

    test('never render a reference that is not a web link', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await panel.getByText('References (1)').click();

        await expect(panel.locator('a[href^="javascript:"]')).toHaveCount(0);
        await expect(panel.getByText('Exploit', {exact: true})).toHaveCount(0);
    });

    test('are absent when there is nothing to show', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='status'
            />,
        );

        await expect(panel.getByText('No vulnerability dataset is installed.').first()).toBeVisible();
        await expect(panel.getByText(/^Affected, /)).toHaveCount(0);
        await expect(panel.getByText(/^References \(/)).toHaveCount(0);
    });
});

test.describe('the hover card', () => {
    test('is one line, and it is the headline', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
            />,
        );

        await expect(card.getByText(HEADLINE)).toBeVisible();
        await expect(card.getByText(SUMMARY)).toHaveCount(0);
    });

    test('renders nothing while the request is in flight', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
                reply='hold'
            />,
        );

        await expect(card.getByTestId('requests')).toBeVisible();
        await expect(card.getByText(HEADLINE)).toHaveCount(0);
    });

    test('renders nothing when the lookup failed', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
                reply='failed'
            />,
        );

        await expect(card.getByTestId('requests')).toHaveText('1');
        await expect(card.getByText(HEADLINE)).toHaveCount(0);
    });

    test('renders nothing when the server refused the link', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
                reply='rejected'
            />,
        );

        await expect(card.getByTestId('requests')).toHaveText('1');
        await expect(card.getByText(HEADLINE)).toHaveCount(0);
    });

    test('shares its answer with the panel that follows it', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
            />,
        );

        await expect(card.getByText(HEADLINE)).toBeVisible();
        await expect(card.getByTestId('requests')).toHaveText('1');
    });
});
