import type {Locator} from '@playwright/test';
import React from 'react';

import CyberHarness from './CyberHarness';

import {expect, test} from '../../../playwright/ct-coverage';

const PUBLISHED_QUERY = 'a=&dtg=101015ZDEC21&t=1639131300000&z=Z';
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
        await expect(panel.getByText('2021-12-10 10:15 UTC')).toBeVisible();
    });

    test('ends with a data sources link that opens a table of every file and when it was compiled', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        const sources = panel.getByTestId('cyber-sources');
        await expect(sources.getByRole('table')).toBeHidden();

        await sources.getByText('Data sources').click();

        const table = sources.getByRole('table');
        await expect(table).toBeVisible();
        await expect(table.getByRole('row')).toHaveCount(3);
        await expect(table.getByText('cve.tsv')).toBeVisible();
        await expect(table.getByText('kev.tsv')).toBeVisible();
        await expect(table.getByText('2026-09-24 13:00 UTC')).toBeVisible();
        await expect(table.getByRole('link', {name: '2026-09-24 01:00 UTC'})).toBeVisible();
    });

    test('shows no data sources link when no file is dated', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='status'
            />,
        );

        await expect(panel.getByText('No vulnerability dataset is installed.').first()).toBeVisible();
        await expect(panel.getByTestId('cyber-sources')).toHaveCount(0);
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

test.describe('the readings', () => {
    test('leave the kind to the sidebar header rather than repeating it', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        await expect(panel.getByText(SUMMARY)).toBeVisible();
        await expect(panel.getByText('Vulnerability', {exact: true})).toHaveCount(0);
    });

    test('decode the vector, marking the most dangerous values', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        const vector = panel.getByTestId('cyber-vector');
        await expect(vector).toContainText('Attack vector');
        await expect(vector.getByText('Network', {exact: true})).toHaveCSS('font-weight', '600');
        await expect(vector.getByText('Required', {exact: true})).toHaveCSS('font-weight', '400');
    });

    test('decode no vector for a record that has none', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
                reply='bare'
            />,
        );

        await expect(panel.getByText('CVE-2021-44228').first()).toBeVisible();
        await expect(panel.getByTestId('cyber-vector')).toHaveCount(0);
    });

    for (const [reply, payload, value] of [
        ['found', CVE, '10.0 Critical'],
        ['weakness', CWE, '10.0 Critical'],
        ['technique', {kind: 'attack', value: 'T1059.001'}, '10.0 Critical'],
        ['address', {kind: 'ip', value: '8.8.8.8'}, 'Mountain View'],
    ] as const) {
        test(`offer no copy button for a ${payload.kind} reading`, async ({mount}) => {
            const panel = await mount(
                <CyberHarness
                    surface='panel'
                    payload={payload}
                    reply={reply}
                />,
            );

            await expect(panel.getByRole('cell', {name: value})).toBeVisible();
            await expect(panel.getByRole('button', {name: /^Copy /})).toHaveCount(0);
        });
    }

    test('render a timestamp as a date-time group link', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CVE}
            />,
        );

        const link = panel.getByRole('link', {name: '2021-12-10 10:15 UTC'});
        await expect(link).toHaveAttribute('href', new RegExp(`/decorate/dtg\\?${PUBLISHED_QUERY.replace(/[?]/g, '\\?')}$`));
        await expect(panel.getByRole('cell', {name: '10.0 Critical'}).getByRole('link')).toHaveCount(0);
    });
});

test.describe('a weakness', () => {
    test('shows its detail in collapsed sections', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CWE}
                reply='weakness'
            />,
        );

        await expect(section(panel, 'Mitigations')).toHaveText('Mitigations1');
        await expect(section(panel, 'Observed examples')).toHaveText('Observed examples2');
        await expect(panel.getByText('Encode it.')).toBeHidden();

        await section(panel, 'Mitigations').click();

        await expect(panel.getByText('Implementation, Output Encoding (effectiveness high)')).toBeVisible();
        await expect(panel.getByText('Encode it.')).toBeVisible();
    });

    test('opens an observed CVE in the sidebar, and leaves a citation as text', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={CWE}
                reply='weakness'
            />,
        );

        await section(panel, 'Observed examples').click();

        await expect(panel.getByRole('button', {name: '[REF-1]'})).toHaveCount(0);
        await expect(panel.getByText('[REF-1]')).toBeVisible();

        await panel.getByRole('button', {name: 'CVE-2021-44228'}).click();
        await expect(panel.getByTestId('selection')).toHaveText('cyber:{"kind":"cve","value":"CVE-2021-44228"}');
    });
});

test.describe('a technique', () => {
    test('links a procedure example out to ATT&CK in a new tab', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={{kind: 'attack', value: 'T1059.001'}}
                reply='technique'
            />,
        );

        await section(panel, 'Procedure examples').click();

        const link = panel.getByRole('link', {name: 'G0007 APT28 (group)'});
        await expect(link).toHaveAttribute('href', 'https://attack.mitre.org/groups/G0007');
        await expect(link).toHaveAttribute('target', '_blank');
        await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
        await expect(panel.getByText('APT28 used PowerShell.')).toBeVisible();
    });

    test('never links an item whose address is not a web link', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={{kind: 'attack', value: 'T1059.001'}}
                reply='technique'
            />,
        );

        await section(panel, 'Procedure examples').click();

        await expect(panel.getByText('Bad Scheme')).toBeVisible();
        await expect(panel.getByRole('link', {name: 'Bad Scheme'})).toHaveCount(0);
        await expect(panel.locator('a[href^="javascript:"]')).toHaveCount(0);
    });
});

test.describe('an address', () => {
    test('credits the vendor whose data it shows, with a link back', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={{kind: 'ip', value: '8.8.8.8'}}
                reply='address'
            />,
        );

        const link = panel.getByRole('link', {name: 'IP Geolocation by DB-IP'});
        await expect(link).toBeVisible();
        await expect(link).toHaveAttribute('href', 'https://db-ip.com');
        await expect(link).toHaveAttribute('target', '_blank');
        await expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    test('shows a credit whose address is not a web link as text', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={{kind: 'ip', value: '8.8.8.8'}}
                reply='address'
            />,
        );

        await expect(panel.getByText('A vendor with a bad address')).toBeVisible();
        await expect(panel.getByRole('link', {name: 'A vendor with a bad address'})).toHaveCount(0);
        await expect(panel.locator('a[href^="javascript:"]')).toHaveCount(0);
    });

    test('credits the vendor in the hover card too', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={{kind: 'ip', value: '8.8.8.8'}}
                reply='address'
            />,
        );

        await expect(card.getByText('AS15169 GOOGLE', {exact: true})).toBeVisible();
        await expect(card.getByText('IP Geolocation by DB-IP')).toBeVisible();
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

        const hosts = await panel.locator('details:not([data-testid="cyber-sources"]) a[href]').evaluateAll((links) => links.map((link) => new URL((link as HTMLAnchorElement).href).host));
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

test.describe('threat reports', () => {
    test('are listed near the top of the panel with their sources', async ({mount}) => {
        const panel = await mount(
            <CyberHarness
                surface='panel'
                payload={{kind: 'ip', value: '8.8.8.8'}}
                reply='address'
            />,
        );

        const reports = panel.getByTestId('cyber-reports');
        await expect(reports.getByRole('link', {name: 'CISA AA99-001A'})).toHaveAttribute('href', 'https://www.cisa.gov/news-events/cybersecurity-advisories/aa99-001a');
        await expect(reports.getByText('Botnet C2', {exact: true})).toHaveCSS('font-weight', '600');
        await expect(reports.getByText('Tor exit node', {exact: true})).toHaveCSS('font-weight', '400');
        await expect(reports.getByRole('link', {name: 'Tor Project'})).toHaveCount(0);
        await expect(panel.locator('a[href^="javascript:"]')).toHaveCount(0);
    });

    test('mark the hover as reported malicious, counting the sources, and name the context', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={{kind: 'ip', value: '8.8.8.8'}}
                reply='address'
            />,
        );

        await expect(card.getByTestId('cyber-reported')).toHaveText('Reported malicious by 2 sources');
        await expect(card.getByTestId('cyber-context')).toHaveText('Tor exit node');
    });

    test('leave the hover without a malicious badge when nothing reports one', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CWE}
                reply='weakness'
            />,
        );

        await expect(card.getByTestId('cyber-glance')).toBeVisible();
        await expect(card.getByTestId('cyber-reported')).toHaveCount(0);
    });
});

test.describe('the glance card', () => {
    test('shows a weakness with its kind, what it affects and its counts', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CWE}
                reply='weakness'
            />,
        );

        const glance = card.getByTestId('cyber-glance');
        await expect(glance.getByText('Cross-site Scripting')).toBeVisible();
        await expect(glance.getByText('CWE-79 · Base · Stable')).toBeVisible();
        await expect(glance.getByText('The product does not neutralize input placed in a web page.')).toBeVisible();
        await expect(glance.getByText('Confidentiality', {exact: true})).toBeVisible();
        await expect(glance.getByText('12 mitigations · 20 observed examples')).toBeVisible();
    });

    test('shows a retired technique and a watchlist verdict as badges', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={{kind: 'attack', value: 'T1059.001'}}
                reply='technique'
            />,
        );

        await expect(card.getByText('Revoked by MITRE, replaced by T1685 Disable or Modify Tools')).toBeVisible();
        await expect(card.getByTestId('cyber-verdict')).toHaveText('Watchlist: suspicious');
        await expect(card.getByText('Defense Impairment', {exact: true})).toBeVisible();
    });

    test('shows an address with its network, scope, place and credit', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={{kind: 'ip', value: '8.8.8.8'}}
                reply='address'
            />,
        );

        const glance = card.getByTestId('cyber-glance');
        await expect(glance.getByText('8.8.8.8', {exact: true})).toBeVisible();
        await expect(glance.getByText('AS15169 GOOGLE', {exact: true})).toBeVisible();
        await expect(glance.getByText('Global', {exact: true})).toBeVisible();
        await expect(glance.getByText('Mountain View, California, US')).toBeVisible();
    });

    test('adds a watchlist verdict to a vulnerability badges', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
            />,
        );

        const glance = card.getByTestId('cyber-glance');
        await expect(glance.getByTestId('cyber-severity')).toBeVisible();
        await expect(glance.getByTestId('cyber-verdict')).toHaveText('Watchlist: malicious');
    });

    test('stays inside the hover width', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CWE}
                reply='weakness'
            />,
        );

        const box = await card.getByTestId('cyber-glance').boundingBox();
        expect(box?.width).toBeLessThanOrEqual(360);
    });
});

test.describe('the hover card', () => {
    test('shows a vulnerability with the badges the panel shows, then what it is and how it is reached', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
            />,
        );

        const glance = card.getByTestId('cyber-glance');
        await expect(glance.getByText('CVE-2021-44228', {exact: true})).toBeVisible();
        await expect(glance.getByTestId('cyber-severity')).toHaveText('10.0Critical');
        await expect(glance.getByTestId('cyber-exploited')).toHaveText('Known exploited');
        await expect(glance.getByText('Published 2021-12-10 · CWE-502')).toBeVisible();
        await expect(glance.getByText(SUMMARY)).toBeVisible();
        await expect(glance.getByText('No user interaction', {exact: true})).toBeVisible();
        await expect(glance.getByText('EPSS 97.5% · KEV due 2021-12-24')).toBeVisible();
        await expect(card.getByText(HEADLINE)).toHaveCount(0);
    });

    test('keeps a vulnerability with only badges to the badges alone', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
                reply='badges'
            />,
        );

        await expect(card.getByTestId('cyber-severity')).toBeVisible();
        await expect(card.getByTestId('cyber-glance')).toHaveCount(0);
    });

    test('keeps the badges on one line', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
            />,
        );

        const severity = await card.getByTestId('cyber-severity').boundingBox();
        const exploited = await card.getByTestId('cyber-exploited').boundingBox();
        expect(severity?.y).toBe(exploited?.y);
    });

    test('falls back to the headline for a vulnerability with no score and no listing', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
                reply='bare'
            />,
        );

        await expect(card.getByText(HEADLINE)).toBeVisible();
        await expect(card.getByTestId('cyber-severity')).toHaveCount(0);
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
        await expect(card.getByTestId('cyber-severity')).toHaveCount(0);
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
        await expect(card.getByTestId('cyber-severity')).toHaveCount(0);
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
        await expect(card.getByTestId('cyber-severity')).toHaveCount(0);
    });

    test('shares its answer with the panel that follows it', async ({mount}) => {
        const card = await mount(
            <CyberHarness
                surface='hover'
                payload={CVE}
            />,
        );

        await expect(card.getByTestId('cyber-severity')).toBeVisible();
        await expect(card.getByTestId('requests')).toHaveText('1');
    });
});
