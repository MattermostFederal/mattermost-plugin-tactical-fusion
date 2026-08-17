import manifest from 'manifest';
import React from 'react';

import PostBodyHarness from './PostBodyHarness';

import {expect, test} from '../../playwright/ct-coverage';

const HREF = `/plugins/${manifest.id}/decorate/fix?f=ddh&v=34.0561N%2C118.2500W`;

const LINK = `[34.0561N,118.2500W](${HREF})`;

test('renders the link and the inline view for a post that is one token', async ({mount}) => {
    const component = await mount(<PostBodyHarness message={LINK}/>);

    await expect(component.getByRole('link', {name: '34.0561N,118.2500W'})).toHaveAttribute('href', HREF);
    await expect(component.getByTestId('fixture-inline')).toBeVisible();
});

// The label the location decorator deliberately does not consume stays in front
// of the link, so a decorated USMTF line still reads as USMTF.
test('keeps a label the decorator left in the message', async ({mount}) => {
    const component = await mount(<PostBodyHarness message={`MGRS: ${LINK}`}/>);

    await expect(component).toContainText('MGRS:');
    await expect(component.getByTestId('fixture-inline')).toBeVisible();
});

/*
 * Post.Type survives an edit unconditionally and Props may not, so a stamped
 * post can arrive with a message that no longer names its payload. Every one of
 * these renders the message and nothing else.
 */
test.describe('a payload that does not match the message', () => {
    test('an edited message drops back to its own text', async ({mount}) => {
        const component = await mount(<PostBodyHarness message='see the plan'/>);

        await expect(component).toHaveText('see the plan');
        await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
    });

    test('props naming a different coordinate are refused', async ({mount}) => {
        const component = await mount(
            <PostBodyHarness
                message={LINK}
                propsV='35.0000N,119.0000W'
            />,
        );

        await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
    });

    test('a post carrying no props of ours is refused', async ({mount}) => {
        const component = await mount(
            <PostBodyHarness
                message={LINK}
                propsF={null}
                propsV={null}
            />,
        );

        await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
    });

    test('a props version this build does not know is refused', async ({mount}) => {
        const component = await mount(
            <PostBodyHarness
                message={LINK}
                propsVersion={99}
            />,
        );

        await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
    });

    test('props naming a different decorator are refused', async ({mount}) => {
        const component = await mount(
            <PostBodyHarness
                message={LINK}
                propsType='other'
            />,
        );

        await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
    });
});

// Compact display is a density choice, and a map contradicts it. The link still
// renders, which is the whole message.
test('draws the link but no inline view in compact display', async ({mount}) => {
    const component = await mount(
        <PostBodyHarness
            message={LINK}
            compactDisplay={true}
        />,
    );

    await expect(component.getByRole('link', {name: '34.0561N,118.2500W'})).toBeVisible();
    await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
});

test('renders the message when the decorator declares no inline view', async ({mount}) => {
    const component = await mount(
        <PostBodyHarness
            message={LINK}
            kind='none'
        />,
    );

    await expect(component).toHaveText(LINK);
});

/*
 * Mattermost wraps a registered post-type component in its own error boundary,
 * and that one replaces the whole post body with an error notice. Since this
 * plugin renders the body, a throw inside the map would otherwise cost the
 * reader the message itself.
 */
test('an inline view that throws leaves the link on screen', async ({mount}) => {
    const component = await mount(
        <PostBodyHarness
            message={LINK}
            kind='throwing'
        />,
    );

    await expect(component.getByRole('link', {name: '34.0561N,118.2500W'})).toBeVisible();
    await expect(component.getByTestId('fixture-inline')).toHaveCount(0);
});
