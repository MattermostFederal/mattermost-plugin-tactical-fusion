import {expect, test} from '@playwright/test';

import {cameraFromHash, hashForCamera} from './camera';
import {MAX_ZOOM, MERCATOR_LIMIT} from './span';

test('a camera round-trips through the fragment', () => {
    const camera = {lat: 34.0561, lon: -118.25, zoom: 12.5};

    expect(cameraFromHash(hashForCamera(camera))).toEqual(camera);
});

test('the fragment is zoom, then latitude, then longitude', () => {
    expect(hashForCamera({lat: 34.0561, lon: -118.25, zoom: 12.5})).
        toBe('#map=12.50/34.05610/-118.25000');
});

test('a longitude past the seam is wrapped rather than carried', () => {
    const wrapped = cameraFromHash(hashForCamera({lat: 0, lon: 190, zoom: 4}));

    expect(wrapped).toEqual({lat: 0, lon: -170, zoom: 4});
});

test('a fragment that names nothing is not a camera', () => {
    expect(cameraFromHash('')).toBeNull();
    expect(cameraFromHash('#')).toBeNull();
    expect(cameraFromHash('#post=abc')).toBeNull();
    expect(cameraFromHash('#map=')).toBeNull();
});

test('a fragment missing a field is refused rather than half read', () => {
    expect(cameraFromHash('#map=12.5/34.05')).toBeNull();
    expect(cameraFromHash('#map=12.5/34.05/-118.25/7')).toBeNull();
});

test('an empty field is refused, which Number() would read as zero', () => {
    expect(cameraFromHash('#map=/34.05/-118.25')).toBeNull();
    expect(cameraFromHash('#map=12.5//-118.25')).toBeNull();
    expect(cameraFromHash('#map=12.5/34.05/')).toBeNull();
});

test('a field that is not a number is refused', () => {
    expect(cameraFromHash('#map=z12/34.05/-118.25')).toBeNull();
    expect(cameraFromHash('#map=Infinity/34.05/-118.25')).toBeNull();
    expect(cameraFromHash('#map=1e9/34.05/-118.25')).toBeNull();
    expect(cameraFromHash('#map=12.5/NaN/-118.25')).toBeNull();
});

test('a zoom outside what the map draws is refused', () => {
    expect(cameraFromHash(`#map=${MAX_ZOOM + 1}/34.05/-118.25`)).toBeNull();
    expect(cameraFromHash('#map=-1/34.05/-118.25')).toBeNull();
    expect(cameraFromHash(`#map=${MAX_ZOOM}/34.05/-118.25`)).not.toBeNull();
});

test('a latitude the projection cannot represent is refused', () => {
    expect(cameraFromHash('#map=8/89.9/12')).toBeNull();
    expect(cameraFromHash('#map=8/-89.9/12')).toBeNull();
    expect(cameraFromHash('#map=8/85.05112/12')).not.toBeNull();
});

test('a camera at the top of the projection still round-trips', () => {
    const top = hashForCamera({lat: MERCATOR_LIMIT, lon: 12, zoom: 8});

    expect(cameraFromHash(top)).not.toBeNull();
});

test('a longitude off the globe is refused', () => {
    expect(cameraFromHash('#map=8/34.05/181')).toBeNull();
    expect(cameraFromHash('#map=8/34.05/-181')).toBeNull();
});
