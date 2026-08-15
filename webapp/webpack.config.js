const exec = require('child_process').exec;
const path = require('path');

const PLUGIN_ID = require('../plugin.json').id;

const NPM_TARGET = process.env.npm_lifecycle_event; //eslint-disable-line no-process-env
const isDev = NPM_TARGET === 'debug' || NPM_TARGET === 'debug:watch';

const plugins = [];
if (NPM_TARGET === 'build:watch' || NPM_TARGET === 'debug:watch') {
    plugins.push({
        apply: (compiler) => {
            compiler.hooks.watchRun.tap('WatchStartPlugin', () => {
                // eslint-disable-next-line no-console
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
                test: /maplibre-gl-worker\.mjs$/,
                type: 'asset/resource',
                generator: {filename: '[name].[contenthash][ext]'},
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
                // Emitted WITHOUT a contenthash, because the name the worker asks
                // for is fixed.
                test: /maplibre-gl-shared\.mjs$/,
                resourceQuery: /copy/,
                type: 'asset/resource',
                generator: {filename: '[name][ext]'},
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
    mode: (isDev) ? 'eval-source-map' : 'production',
    plugins,
};

const config = shared;

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
    output: {
        devtoolNamespace: PLUGIN_ID + '-page',
        path: path.join(__dirname, '../public/app'),
        filename: '[name].js',
        chunkFilename: '[name].[contenthash].js',
        publicPath: 'auto',
    },
    plugins: [],
};

module.exports = [config, pageConfig];
