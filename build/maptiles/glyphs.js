const fs = require('fs');
const path = require('path');

const fontnik = require('fontnik');

/*
 * Signed-distance-field glyph ranges for the map's labels.
 *
 * MapLibre will not draw a label without these. Its local-font fallback covers
 * ideographic text only, so a missing range costs Latin labels entirely rather
 * than costing them their typeface: with no fonts served, the label features
 * arrive in the tiles and draw as nothing at all.
 *
 * The ranges are the four that cover every Latin script Natural Earth's ADMIN
 * field uses, plus the punctuation block that carries the apostrophe in names
 * like Cote d'Ivoire. A full plane would be about 10 MB per face; these four are
 * about a quarter of a megabyte.
 */

const FONT = '/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf';
const OUT = 'public/map/fonts/NotoSans-Regular';

const RANGES = [
    [0, 255],       // Basic Latin, Latin-1 Supplement
    [256, 511],     // Latin Extended-A
    [512, 767],     // Latin Extended-B
    [8192, 8447],   // General Punctuation
];

function range(font, start, end) {
    return new Promise((resolve, reject) => {
        fontnik.range({font, start, end}, (err, data) => (err ? reject(err) : resolve(data)));
    });
}

async function main() {
    const font = fs.readFileSync(FONT);
    fs.mkdirSync(OUT, {recursive: true});

    for (const [start, end] of RANGES) {
        const data = await range(font, start, end);
        const file = path.join(OUT, `${start}-${end}.pbf`);

        fs.writeFileSync(file, data);
        console.log(`${file}  ${data.length} bytes`);
    }
}

main().catch((err) => {
    console.error(err);
    process.exit(1);
});
