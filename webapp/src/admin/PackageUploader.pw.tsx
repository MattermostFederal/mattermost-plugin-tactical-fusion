import type {Locator, Page} from '@playwright/test';
import React from 'react';

import {REFUSAL} from './package_fixtures';
import PackageUploaderHarness from './PackageUploaderHarness';

import {expect, test} from '../../playwright/ct-coverage';

/*
 * The System Console package control.
 *
 * The one setting in this plugin that writes to disk, and the only one whose
 * failures an admin meets before anything works: a name that is not an area, a
 * file past what the route carries, a proxy that drops a large POST. Each of
 * those has to say which one it was, because the recovery differs and the
 * directory this writes into is reachable by hand.
 */

const EMPTY = 'No detail areas are installed. The map draws its global basemap everywhere.';
const BUNDLED = 'ships with the plugin';
const ARCHIVE = 'eucom-baltics.pmtiles';

function fileInput(component: Locator): Locator {
    return component.locator('input[type=file]');
}

async function choose(component: Locator, name: string): Promise<void> {
    await fileInput(component).setInputFiles({
        name,
        mimeType: 'application/octet-stream',
        buffer: Buffer.from('PMTiles'),
    });
}

/*
 * A file the control will read as larger than the ceiling.
 *
 * setInputFiles cannot hand over half a gigabyte, so the size is defined onto a
 * real File and that File onto the input. The handler still reads
 * event.target.files[0] and still asks it for its size, so the path under test
 * is the component's own; only the file is synthetic.
 */
async function chooseOversized(page: Page, name: string, megabytes: number): Promise<void> {
    await page.evaluate(({fileName, size}) => {
        const input = document.querySelector('input[type=file]') as HTMLInputElement;
        const file = new File(['x'], fileName, {type: 'application/octet-stream'});
        Object.defineProperty(file, 'size', {value: size});

        Object.defineProperty(input, 'files', {
            configurable: true,
            value: {0: file, length: 1, item: (i: number) => (i === 0 ? file : null)},
        });

        input.dispatchEvent(new Event('change', {bubbles: true}));
    }, {fileName: name, size: megabytes * 1024 * 1024});
}

test.describe('the installed list', () => {
    test('says what the map does instead when no area is installed', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);

        await expect(component.getByText(EMPTY)).toBeVisible();
    });

    test('lists every installed area', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='ok'/>);

        await expect(component.getByText('indopacom-guam', {exact: true})).toBeVisible();
        await expect(component.getByText('indopacom-hawaii', {exact: true})).toBeVisible();
        await expect(component.getByText(EMPTY)).toBeHidden();
    });

    /*
     * Remove is offered for what Remove can do. A bundled area is inside the
     * plugin and a release replaces it, so a button beside it could only ever
     * report a failure.
     */
    test('offers Remove for a dropped-in area and says so for a bundled one', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='ok'/>);

        const dropIn = component.locator('li').filter({hasText: 'indopacom-guam'});
        await expect(dropIn.getByRole('button', {name: 'Remove'})).toBeVisible();

        const bundled = component.locator('li').filter({hasText: 'indopacom-hawaii'});
        await expect(bundled.getByText(BUNDLED)).toBeVisible();
        await expect(bundled.getByRole('button', {name: 'Remove'})).toBeHidden();
    });

    // A payload carrying anything but names is a server this build does not
    // know, and rendering [object Object] as an area name is worse than
    // dropping it.
    test('drops anything in the payload that is not a name', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='mixed-types'/>);

        await expect(component.getByText('indopacom-hawaii', {exact: true})).toBeVisible();
        await expect(component.locator('li')).toHaveCount(1);
    });

    // The directory is the truth and this list is a convenience, so a route an
    // admin cannot reach leaves the screen it found rather than an error.
    const unreadable = {
        refused: 'the route refuses the reader',
        offline: 'the request never arrives',
        'not-an-array': 'the payload is not a list of names',
    } as const;

    for (const [list, because] of Object.entries(unreadable)) {
        test(`leaves the empty state when ${because}`, async ({mount}) => {
            const component = await mount(<PackageUploaderHarness list={list as keyof typeof unreadable}/>);

            await expect(component.getByText(EMPTY)).toBeVisible();
        });
    }
});

test.describe('uploading', () => {
    /*
     * The name comes from the FILE rather than from a field, because the name
     * is what the map is keyed on: asking separately would let an admin upload
     * one area under another area's name and then wonder why what they built is
     * not what they see.
     */
    test('posts the archive under the name the file carries', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);
        await choose(component, ARCHIVE);

        await expect(component.getByTestId('requests')).toContainText('POST /plugins/');
        await expect(component.getByTestId('requests')).toContainText('/api/v1/packages/eucom-baltics');
    });

    test('replaces the list with what the server sends back', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);
        await choose(component, ARCHIVE);

        await expect(component.getByText('eucom-baltics', {exact: true})).toBeVisible();
        await expect(component.getByText(EMPTY)).toBeHidden();
    });

    /*
     * An upload lands in the drop-in directory, so the area it just installed
     * is one Remove can unlink. The write answer carries `removable` for that
     * reason, and reading only `packages` from it left the row claiming to ship
     * inside the plugin, one screen after an admin put it there by hand.
     */
    test('offers Remove for the area it just installed', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);
        await choose(component, ARCHIVE);

        const installed = component.locator('li').filter({hasText: 'eucom-baltics'});
        await expect(installed.getByRole('button', {name: 'Remove'})).toBeVisible();
        await expect(installed.getByText(BUNDLED)).toBeHidden();

        const bundled = component.locator('li').filter({hasText: 'indopacom-hawaii'});
        await expect(bundled.getByText(BUNDLED)).toBeVisible();
    });

    test('refuses a file that is not named for an area, without asking the server', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);
        await choose(component, 'Baltics.pmtiles');

        await expect(component.getByText('"Baltics.pmtiles" is not named <command>-<area>.pmtiles, such as indopacom-hawaii.pmtiles.')).toBeVisible();
        await expect(component.getByTestId('requests')).not.toContainText('POST');
    });

    // Bigger than the route carries, so the answer is the other door into the
    // same directory rather than a failed upload with no way forward.
    test('refuses a file past the ceiling and says to copy it in instead', async ({mount, page}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);
        await chooseOversized(page, ARCHIVE, 600);

        await expect(component.getByText(`${ARCHIVE} is larger than 512 MB. Copy it into the package directory instead.`)).toBeVisible();
        await expect(component.getByTestId('requests')).not.toContainText('POST');
    });

    test('shows the reason the server gave', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='empty'
            write='refused'
                                      />);
        await choose(component, ARCHIVE);

        await expect(component.getByText(REFUSAL)).toBeVisible();
    });

    test('says the upload was refused when the server did not say why', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='empty'
            write='refused-silently'
                                      />);
        await choose(component, ARCHIVE);

        await expect(component.getByText('The upload was refused.')).toBeVisible();
    });

    /*
     * A POST that never arrives is most often a proxy or a body limit in front
     * of Mattermost rather than the plugin, and neither sends a message this
     * could show, so the fallback names the size and the way around it.
     */
    test('points at the directory when the upload cannot be completed', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='empty'
            write='offline'
                                      />);
        await choose(component, ARCHIVE);

        await expect(component.getByText(/A large package may exceed what this server accepts/)).toBeVisible();
    });

    test('names what it is installing and takes nothing else while it does', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='empty'
            write='hold'
                                      />);
        await choose(component, ARCHIVE);

        await expect(component.getByText('Installing eucom-baltics…')).toBeVisible();
        await expect(fileInput(component)).toBeDisabled();
    });

    test('does nothing when no file is chosen', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness list='empty'/>);
        await fileInput(component).setInputFiles([]);

        await expect(component.getByText(EMPTY)).toBeVisible();
        await expect(component.getByTestId('requests')).not.toContainText('POST');
    });
});

test.describe('removing', () => {
    test('sends a DELETE and takes the list the server sends back', async ({mount}) => {
        const component = await mount(
            <PackageUploaderHarness
                list='ok'
                result='removed'
            />);
        await component.getByRole('button', {name: 'Remove'}).click();

        await expect(component.getByTestId('requests')).toContainText('DELETE /plugins/');
        await expect(component.getByTestId('requests')).toContainText('/api/v1/packages/indopacom-guam');
        await expect(component.getByText('indopacom-guam', {exact: true})).toBeHidden();
    });

    test('shows the reason the server gave', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='ok'
            write='refused'
                                      />);
        await component.getByRole('button', {name: 'Remove'}).click();

        await expect(component.getByText(REFUSAL)).toBeVisible();
        await expect(component.getByText('indopacom-guam', {exact: true})).toBeVisible();
    });

    test('says the package could not be removed when the server did not say why', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='ok'
            write='refused-silently'
                                      />);
        await component.getByRole('button', {name: 'Remove'}).click();

        await expect(component.getByText('The package could not be removed.')).toBeVisible();
    });

    test('says the same when the request never arrives', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='ok'
            write='offline'
                                      />);
        await component.getByRole('button', {name: 'Remove'}).click();

        await expect(component.getByText('The package could not be removed.')).toBeVisible();
    });

    /*
     * Removing a drop-in that shadowed a bundled area of the same name leaves
     * the name listed, because the bundled one underneath resurfaces. It is no
     * longer removable, and a Remove button still offered there could only ever
     * report a failure, which is the shape this control already had once.
     */
    test('stops offering Remove once only the bundled area is left', async ({mount}) => {
        const component = await mount(
            <PackageUploaderHarness
                list='shadowing'
                result='removed'
            />);
        await component.getByRole('button', {name: 'Remove'}).click();

        await expect(component.getByText('indopacom-hawaii', {exact: true})).toBeVisible();
        await expect(component.getByRole('button', {name: 'Remove'})).toBeHidden();
        await expect(component.getByText(BUNDLED)).toBeVisible();
    });

    // One write at a time, so a second click cannot race the first into the
    // same directory.
    test('takes nothing else while a removal is in flight', async ({mount}) => {
        const component = await mount(<PackageUploaderHarness
            list='ok'
            write='hold'
                                      />);
        await component.getByRole('button', {name: 'Remove'}).click();

        await expect(component.getByRole('button', {name: 'Remove'})).toBeDisabled();
        await expect(fileInput(component)).toBeDisabled();
    });
});

/*
 * The note is the only instruction an operator gets for the path this control
 * cannot take: an area too large to upload, and a cluster where an upload
 * reaches one node.
 */
test('the note names the ceiling, the shape of a name and the cluster case', async ({mount}) => {
    const component = await mount(<PackageUploaderHarness list='empty'/>);

    await expect(component.getByText(/512/)).toBeVisible();
    await expect(component.getByText('<command>-<area>.pmtiles', {exact: true})).toBeVisible();
    await expect(component.getByText(/an upload reaches only the node that served it/)).toBeVisible();
});
