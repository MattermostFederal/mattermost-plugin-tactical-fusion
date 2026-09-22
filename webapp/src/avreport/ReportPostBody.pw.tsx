import React from 'react';

import {HONOLULU_METAR, NOTAM_WITH_RADIUS} from './report_fixtures';
import ReportPostBodyHarness from './ReportPostBodyHarness';

import {expect, test} from '../../playwright/ct-coverage';
import {stubFeaturesRoute} from '../features/stub_route';

test.beforeEach(async ({page}) => {
    await stubFeaturesRoute(page, {mapPanel: true, mapInline: false, mapPage: true});
});

test('renders the report as posted, its summary and its decoded rows', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await expect(body.getByTestId('avreport-card')).toBeVisible();
    await expect(body.getByTestId('avreport-heading')).toHaveText('METAR PHNL');
    await expect(body.getByTestId('avreport-source')).toHaveText(HONOLULU_METAR.src);
    await expect(body.getByTestId('avreport-summary')).toContainText('Wind 070°');
    await expect(body.getByTestId('avreport-rows')).toContainText('Issued');
    await expect(body.getByTestId('avreport-rows')).toContainText('month and year taken from the post date');
    await expect(body.getByTestId('avreport-rows')).toContainText('30.10 inHg');
    await expect(body.getByTestId('avreport-remarks')).toContainText('automated station');
    await expect(body.getByTestId('avreport-unknown')).toHaveText('Q9999');
    await expect(body.getByRole('button', {name: 'Daniel K. Inouye International Airport'})).toBeVisible();
});

test('the station name opens the airfield panel', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await body.getByRole('button', {name: 'Daniel K. Inouye International Airport'}).click();

    await expect(body.getByTestId('selection')).toHaveText('airport {"key":"icao","code":"PHNL"}');
});

test('Open details opens the report panel', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await body.getByRole('button', {name: 'Open details'}).click();

    await expect(body.getByTestId('selection')).toContainText('avreport-post');
});

test('a multi-line NOTAM keeps its lines', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness payload={NOTAM_WITH_RADIUS}/>);

    await expect(body.getByTestId('avreport-heading')).toHaveText('NOTAM PHNL');
    await expect(body.getByTestId('avreport-source')).toContainText('Q) PHZH/QMRLC');
    await expect(body.getByTestId('avreport-rows')).toContainText('Effective');
    await expect(body.getByTestId('avreport-unknown')).toBeHidden();
});

test('says when the rows were dropped', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness payload={{...HONOLULU_METAR, rowsDropped: true, rows: [], remarks: [], unknown: []}}/>);

    await expect(body.getByTestId('avreport-degraded')).toBeVisible();
    await expect(body.getByTestId('avreport-summary')).toBeVisible();
});

test('compact display keeps the report and drops the detail', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness compactDisplay={true}/>);

    await expect(body.getByTestId('avreport-source')).toBeVisible();
    await expect(body.getByTestId('avreport-rows')).toBeHidden();
});

test('falls back to the text without props', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness payload={null}/>);

    await expect(body.getByTestId('avreport-card')).toBeHidden();
    await expect(body).toContainText('```metar');
});

test('falls back to the text with a version it cannot read', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness version={99}/>);

    await expect(body.getByTestId('avreport-card')).toBeHidden();
});

test('stands down after an edit', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness editAt={5}/>);

    await expect(body.getByTestId('avreport-card')).toBeHidden();
    await expect(body).toContainText(HONOLULU_METAR.src);
});

test('draws no map when the inline map is off', async ({mount}) => {
    const body = await mount(<ReportPostBodyHarness/>);

    await expect(body.getByTestId('avreport-card')).toBeVisible();
    await expect(body.getByTestId('avreport-map')).toBeHidden();
});
