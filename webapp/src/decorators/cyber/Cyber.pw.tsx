import type {Locator} from '@playwright/test';
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
        await expect(panel.getByRole('cell', {name: '10.0 Critical'})).toBeVisible();
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

function section(panel: Locator, title: string): Locator {
    return panel.locator('summary', {hasText: title});
}

test.describe('the header', () => {
    test('shows the severity and the known exploited listing as badges', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText('Vulnerability', {exact: true})).toBeVisible();
        await expect(panel.getByTestId('cyber-severity')).toHaveText('10.0Critical');
        await expect(panel.getByTestId('cyber-exploited')).toHaveText('Known exploited');
    });

    test('shows no badge for a record with no score and no listing', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='bare'
            />,
        );

        await expect(panel.getByText('CVE-2021-44228').first()).toBeVisible();
        await expect(panel.getByTestId('cyber-severity')).toHaveCount(0);
        await expect(panel.getByTestId('cyber-exploited')).toHaveCount(0);
    });

    test('clamps a long description until the reader asks for the rest', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='long'
            />,
        );

        await expect(panel.getByRole('button', {name: 'Show more'})).toBeVisible();
        await panel.getByRole('button', {name: 'Show more'}).click();
        await expect(panel.getByRole('button', {name: 'Show less'})).toBeVisible();
    });

    test('does not offer more of a short description', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText(SUMMARY)).toBeVisible();
        await expect(panel.getByRole('button', {name: 'Show more'})).toHaveCount(0);
    });

    test('names a related weakness by its identifier and its name', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        const link = panel.getByRole('button', {name: 'CWE-502 Deserialization of Untrusted Data'});
        await expect(link).toBeVisible();
        await expect(link).toHaveCSS('text-align', 'left');
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

        await expect(section(panel, 'Affected, as reported')).toHaveText('Affected, as reported1');
        await expect(section(panel, 'Affected, per NVD')).toHaveText('Affected, per NVD2');
        await expect(section(panel, 'References')).toHaveText('References3');
        await expect(panel.getByText('apache log4j: from 2.0 before 2.3.1, from 2.4 before 2.12.2')).toBeHidden();
    });

    test('open to show every line', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await section(panel, 'Affected, per NVD').click();

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

        await section(panel, 'References').click();

        const link = panel.getByRole('link', {name: 'logging.apache.org/log4j/2.x/security.html'});
        await expect(link).toHaveAttribute('href', 'https://logging.apache.org/log4j/2.x/security.html');
        await expect(link).toHaveAttribute('title', 'https://logging.apache.org/log4j/2.x/security.html');
        await expect(link).toHaveAttribute('target', '_blank');
        await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
        await expect(panel.getByText('Vendor Advisory', {exact: true})).toBeVisible();
        await expect(panel.getByText('Patch', {exact: true})).toBeVisible();
    });

    test('list the vendor advisory and the patch before the rest', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await section(panel, 'References').click();

        const hosts = await panel.locator('details a[href]').evaluateAll((links) => links.map((link) => new URL((link as HTMLAnchorElement).href).host));
        expect(hosts).toEqual(['logging.apache.org', 'packetstormsecurity.com', 'lists.debian.org']);
    });

    test('never render a reference that is not a web link', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await section(panel, 'References').click();

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
        await expect(panel.locator('summary')).toHaveCount(0);
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
