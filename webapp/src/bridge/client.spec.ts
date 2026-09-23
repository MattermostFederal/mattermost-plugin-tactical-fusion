import {expect, test} from '@playwright/test';
import manifest from 'manifest';

import {
    LINK_CACHE_LIMIT,
    LINK_CACHE_TTL_MS,
    TacticalFusionError,
    _resetForTesting,
    _setClockForTesting,
    decorate,
    link,
} from './client';

interface Sent {
    url: string;
    init: RequestInit;
}

function reply(status: number, body: unknown): Promise<Response> {
    return Promise.resolve({
        ok: status >= 200 && status < 300,
        status,
        json: () => Promise.resolve(body),
    } as Response);
}

function stubFetch(answer: (sent: Sent) => Promise<Response>): Sent[] {
    const sent: Sent[] = [];
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
        const call = {url: String(input), init: init ?? {}};
        sent.push(call);
        return answer(call);
    }) as typeof fetch;
    return sent;
}

const PHIK = {
    markdown: '[PHIK](/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK)',
    url: '/plugins/com.mattermost.plugin-tactical-fusion/decorate/airport?v=PHIK',
    type: 'airport',
    label: 'PHIK',
    fits_post: true,
};

const PHIK_LINK = {markdown: PHIK.markdown, url: PHIK.url, type: PHIK.type, label: PHIK.label, fitsPost: true};

test.beforeEach(() => {
    _resetForTesting();
});

test('decorate posts the message to the session route and reads the answer', async () => {
    const sent = stubFetch(() => reply(200, {message: 'decorated', changed: true, fits_post: false}));

    const result = await decorate('plain', {referenceTime: new Date(Date.UTC(2026, 2, 1))});

    expect(result).toEqual({message: 'decorated', changed: true, fitsPost: false});
    expect(sent).toHaveLength(1);
    expect(sent[0].url).toBe(`/plugins/${manifest.id}/api/v1/decorate`);
    expect(sent[0].init.method).toBe('POST');
    expect(sent[0].init.credentials).toBe('same-origin');
    expect((sent[0].init.headers as Record<string, string>)['X-Requested-With']).toBe('XMLHttpRequest');
    expect(JSON.parse(String(sent[0].init.body))).toEqual({message: 'plain', reference_time: Date.UTC(2026, 2, 1)});
});

test('an unusable reference time is left for the server to default', async () => {
    const sent = stubFetch(() => reply(200, {message: 'm', changed: false, fits_post: true}));

    await decorate('m', {referenceTime: Number.NaN});

    expect(JSON.parse(String(sent[0].init.body))).toEqual({message: 'm'});
});

test('link sends the type, token and label', async () => {
    const sent = stubFetch(() => reply(200, PHIK));

    const result = await link('airport', 'PHIK', {label: 'Hickam'});

    expect(result).toEqual(PHIK_LINK);
    expect(sent[0].url).toBe(`/plugins/${manifest.id}/api/v1/link`);
    expect(JSON.parse(String(sent[0].init.body))).toEqual({type: 'airport', token: 'PHIK', label: 'Hickam'});
});

test('a declined link rejects with the code, status and reason', async () => {
    stubFetch(() => reply(422, {message: 'Declined. (TF-19007)', code: 19007, reason: 'disabled'}));

    const error = await link('location', '11S 384640E 3769080N').catch((caught: unknown) => caught);

    expect(error).toBeInstanceOf(TacticalFusionError);
    expect(error).toMatchObject({status: 422, code: 19007, reason: 'disabled', message: 'Declined. (TF-19007)'});
});

test('a reason this build does not know is reported as none', async () => {
    stubFetch(() => reply(422, {message: 'Declined.', code: 19099, reason: 'something_new'}));

    const error = await link('dtg', 'x').catch((caught: unknown) => caught);

    expect(error).toMatchObject({code: 19099, reason: null});
});

test('a response that is not a link rejects', async () => {
    stubFetch(() => reply(200, {markdown: 'x'}));

    await expect(link('airport', 'PHIK')).rejects.toThrow('The server sent no url.');
});

test('a repeated link is served from the cache', async () => {
    const sent = stubFetch(() => reply(200, PHIK));

    await link('airport', 'PHIK');
    await link('airport', 'PHIK');
    await link('airport', 'PHIK', {label: 'Hickam'});

    expect(sent).toHaveLength(2);
});

test('the cache lapses', async () => {
    let clock = 1_000;
    _setClockForTesting(() => clock);
    const sent = stubFetch(() => reply(200, PHIK));

    await link('airport', 'PHIK');
    clock += LINK_CACHE_TTL_MS;
    await link('airport', 'PHIK');

    expect(sent).toHaveLength(2);
});

test('a decline is cached and a failure is not', async () => {
    let answer = reply(422, {message: 'No.', code: 19006, reason: 'not_recognized'});
    const sent = stubFetch(() => answer);

    await link('airport', 'ZZZZ').catch(() => null);
    await link('airport', 'ZZZZ').catch(() => null);
    expect(sent).toHaveLength(1);

    answer = Promise.reject(new Error('offline'));
    await link('airport', 'KIND').catch(() => null);
    answer = reply(200, PHIK);
    await expect(link('airport', 'KIND')).resolves.toEqual(PHIK_LINK);
    expect(sent).toHaveLength(3);
});

test('a caller passing a signal gets a request of its own', async () => {
    const sent = stubFetch(() => reply(200, PHIK));

    await link('airport', 'PHIK');
    await link('airport', 'PHIK', {signal: new AbortController().signal});

    expect(sent).toHaveLength(2);
});

test('the cache is bounded', async () => {
    const sent = stubFetch(() => reply(200, PHIK));

    for (let i = 0; i <= LINK_CACHE_LIMIT; i++) {
        // eslint-disable-next-line no-await-in-loop
        await link('airport', `T${i}`);
    }
    await link('airport', 'T0');

    expect(sent).toHaveLength(LINK_CACHE_LIMIT + 2);
});
