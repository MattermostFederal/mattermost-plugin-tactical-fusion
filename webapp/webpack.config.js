const exec = require('child_process').exec;
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const webpack = require('webpack');

const PLUGIN_ID = require('../plugin.json').id;

const NPM_TARGET = process.env.npm_lifecycle_event;
const isDev = NPM_TARGET === 'debug' || NPM_TARGET === 'debug:watch';

/*
 * A short digest of the bundled basemap, for the archive's cache buster.
 *
 * The plugin version is NOT enough on its own, and the failure is specific.
 * Mattermost re-extracts the bundle on every install, so world.pmtiles gets a
 * fresh modification time each time even when its bytes are identical, and it
 * is served out of public/ where the plugin sets no headers: no ETag, no
 * Cache-Control, just Last-Modified. A browser holding cached byte ranges then
 * revalidates with `If-Range: <old time>`, the validator no longer matches, and
 * the server answers a 16 KB range request with HTTP 200 and the whole 43 MB
 * archive. The basemap probe rejects that, and every map reports that it could
 * not be loaded until the reader clears their cache.
 *
 * Keying the URL on the CONTENT rather than the version fixes it: a rebuild
 * that changes nothing keeps the URL, and a rebuild that changes the archive
 * moves it, so a browser never revalidates a range against a validator that has
 * moved underneath it.
 *
 * Missing is not fatal here. `make bundle` is what enforces that the archive
 * ships; a webpack build without it is a webapp-only build, and falling back to
 * the version keeps that working.
 */
function basemapDigest() {
    const archive = path.resolve(__dirname, '../public/map/world.pmtiles');

    try {
        return crypto.createHash('sha256').update(fs.readFileSync(archive)).digest('hex').slice(0, 16);
    } catch {
        return '';
    }
}

/*
 * The directory MapLibre's worker and its shared chunk are emitted into.
 *
 * Keyed on the CONTENT of both files, and that is the whole point. The worker
 * is content-hashed, but it imports "./maplibre-gl-shared.mjs" by that literal
 * relative name, so the shared chunk has to keep a fixed filename. Mattermost
 * serves /static/plugins/** with `Cache-Control: max-age=31556926`, a year, so
 * a fixed name under that policy is a chunk a browser will not re-fetch until
 * long after it has stopped matching the worker beside it. An upgraded MapLibre
 * then pairs a fresh worker with a year-old shared chunk, the worker fails to
 * start, and the map sits on "Loading map…" with no error: the same silent
 * failure the worker's own comment describes, arriving by a different route.
 *
 * A hashed DIRECTORY fixes what a hashed filename cannot. The worker's relative
 * import resolves inside whatever directory the worker was loaded from, so
 * moving the pair together keeps the name the worker asks for while making the
 * URL change whenever either file does.
 */
function maplibreAssetDir() {
    const digest = crypto.createHash('sha256');

    for (const name of ['maplibre-gl-worker.mjs', 'maplibre-gl-shared.mjs']) {
        digest.update(fs.readFileSync(require.resolve(`maplibre-gl/dist/${name}`)));
    }

    return `maplibre-${digest.digest('hex').slice(0, 12)}`;
}

const MAPLIBRE_DIR = maplibreAssetDir();

const plugins = [

    // A bare identifier rather than process.env.*, because there is no `process`
    // in a browser: guarding the read with `typeof process === 'undefined'`
    // made the digest unreachable at runtime and the URL silently fell back to
    // the plugin version, which is the whole defect this exists to fix.
    new webpack.DefinePlugin({
        __TF_BASEMAP_DIGEST__: JSON.stringify(basemapDigest()),
    }),
];
if (NPM_TARGET === 'build:watch' || NPM_TARGET === 'debug:watch') {
    plugins.push({
        apply: (compiler) => {
            compiler.hooks.watchRun.tap('WatchStartPlugin', () => {
                console.log('Change detected. Rebuilding webapp.');
            });
            compiler.hooks.afterEmit.tap('AfterEmitPlugin', () => {
                exec('cd .. && make deploy-from-watch', (err, stdout, stderr) => {
                    if (stdout) {
                        process.stdout.write(stdout);
                    }
                    if (stderr) {
                        process.stderr.write(stderr);
                    }
                });
            });
        },
    });
}

const shared = {
    entry: [
        './src/index.tsx',
    ],
    resolve: {
        modules: [
            'src',
            'node_modules',
        ],
        extensions: ['*', '.js', '.jsx', '.ts', '.tsx'],
    },
    module: {
        rules: [
            {

                // MapLibre builds its worker from a blob: URL by default, which
                // needs `worker-src blob:` in whatever CSP Mattermost serves the
                // webapp under. Emitting the worker as a real file and handing
                // MapLibre its URL takes a plain same-origin `new Worker(url)`
                // path instead, so a hardened host policy cannot break the map.
                // Into the hashed directory, keeping the plain name: the
                // directory is what carries the version now, and it has to,
                // because the shared chunk beside it cannot be renamed.
                test: /maplibre-gl-worker\.mjs$/,
                type: 'asset/resource',
                generator: {filename: `${MAPLIBRE_DIR}/[name][ext]`},
            },
            {

                // The worker is a module, and it imports "./maplibre-gl-shared.mjs"
                // by that literal relative name. Emitting the worker alone leaves
                // that import a 404, and the failure is silent: the worker never
                // starts, the style never finishes, and the map sits on
                // "Loading map…" forever with no error event.
                //
                // Matched on ?copy, NOT on the filename alone. MapLibre's own
                // main-thread code imports the same file for its code, and
                // treating that import as an asset replaces the module with a URL
                // string, so the shader source is then parsed as JavaScript and
                // the bundle dies on "Unexpected identifier 'precision'".
                //
                // Still emitted under its own fixed name, because that is the
                // name the worker asks for. The DIRECTORY is what makes the URL
                // move when the file does; see maplibreAssetDir.
                test: /maplibre-gl-shared\.mjs$/,
                resourceQuery: /copy/,
                type: 'asset/resource',
                generator: {filename: `${MAPLIBRE_DIR}/[name][ext]`},
            },
            {
                test: /\.(js|jsx|ts|tsx)$/,
                exclude: /node_modules/,
                use: {
                    loader: 'babel-loader',
                    options: {
                        cacheDirectory: true,

                        // Babel configuration is in babel.config.js because jest requires it to be there.
                    },
                },
            },
            {
                test: /\.(scss|css)$/,
                use: [
                    'style-loader',
                    {
                        loader: 'css-loader',
                    },
                    {
                        loader: 'sass-loader',
                        options: {
                            sassOptions: {
                                includePaths: ['node_modules/compass-mixins/lib', 'sass'],
                            },
                        },
                    },
                ],
            },
        ],
    },
    externals: {
        react: 'React',
        'react-dom': 'ReactDOM',
        redux: 'Redux',
        'react-redux': 'ReactRedux',
        'prop-types': 'PropTypes',
        'react-bootstrap': 'ReactBootstrap',
        'react-router-dom': 'ReactRouterDom',
    },
    output: {
        devtoolNamespace: PLUGIN_ID,
        path: path.join(__dirname, '/dist'),
        filename: 'main.js',

        // Lazy chunks land beside main.js. Mattermost copies the whole
        // directory containing bundle_path into its static plugin directory
        // and renames only main.js, so a sibling here is already served at
        // /static/plugins/<id>/. contenthash so an upgraded plugin cannot load
        // a stale chunk against a fresh bundle.
        //
        // publicPath is set at runtime in index.tsx rather than here: the
        // basename is only known in the browser, and a build-time '/' is wrong
        // on every subpath install.
        chunkFilename: '[name].[contenthash].js',
    },
    performance: {
        maxAssetSize: 1024 * 1024,
        maxEntrypointSize: 1024 * 1024,
    },

    // 'eval-source-map' is a devtool value, not a mode, and sat here reading as
    // one. Harmless only because every npm script passes --mode on the CLI,
    // which overrides it.
    mode: (isDev) ? 'development' : 'production',
    plugins,
};

// A copy, not an alias. This used to be `const config = shared`, so the
// Object.assign below mutated `shared` itself and the page build, spread from it
// afterwards, silently inherited `devtool: 'eval-source-map'`. That is eval, and
// the pages are served under script-src 'self' with no 'unsafe-eval', so a debug
// build produced a page bundle its own policy refused to run.
const config = {...shared};

if (isDev) {
    Object.assign(config, {devtool: 'eval-source-map'});
}

/*
 * The standalone pages, built from the same source as the panel.
 *
 * A second configuration rather than a second entry, for two reasons that both
 * have to hold at once: the output goes somewhere else, and `externals` cannot.
 * Mattermost hands the plugin bundle React, Redux and the rest as globals, and
 * a page served from /decorate or /map has no Mattermost webapp around it, so
 * this build has to carry its own copy of them.
 *
 * publicPath is 'auto', which resolves lazy chunks and the MapLibre worker
 * against this script's own URL. The page renderers are pure functions of a
 * query string and cannot see SiteURL, so deriving the base from where the file
 * actually is beats anything they could have passed in.
 */
const pageConfig = {
    ...shared,
    entry: {page: './src/page/index.tsx'},
    externals: {},

    // Explicitly OFF, and this is the whole reason the copy above exists rather
    // than a second half of it. Removing the inherited `devtool` assignment was
    // not enough: webpack DERIVES `devtool: 'eval'` from `mode: 'development'`,
    // and `debug:watch` passes --mode=development, so the page bundle still came
    // out eval-based. The pages are served under script-src 'self' with no
    // 'unsafe-eval', so their own policy refused to run them and the reader got
    // the empty document TestBothPagesNameTheBundleRelativeToTheirOwnRoute
    // exists to prevent, in the one build where somebody is trying to debug it.
    devtool: false,
    output: {
        devtoolNamespace: PLUGIN_ID + '-page',
        path: path.join(__dirname, '../public/app'),
        filename: '[name].js',
        chunkFilename: '[name].[contenthash].js',
        publicPath: 'auto',

        // `make bundle` copies public/ wholesale, so without this every
        // content-hashed chunk from every previous build ships forever.
        clean: true,
    },
    plugins,
};

module.exports = [config, pageConfig];
