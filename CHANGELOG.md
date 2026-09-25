# Changelog

## [0.8.2](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.8.1...v0.8.2) (2026-09-25)


### Bug Fixes

* **build:** accept ClamAV's false positive on the bundled ATT&CK text and build maps after the plugin ([#59](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/59)) ([490dce1](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/490dce1cad065311e03742d5e435e295460f92c0))

## [0.8.1](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.8.0...v0.8.1) (2026-09-25)


### Bug Fixes

* **build:** ship the committed CISA advisory indicators instead of re-fetching them at release ([#57](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/57)) ([015eef4](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/015eef4c9de659a7e1e5c35e8b4e49c1ab168434))

## [0.8.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.7.0...v0.8.0) (2026-09-25)


### Features

* add the cyber context decorator ([#50](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/50)) ([1fba0ca](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/1fba0ca71baf2d33a516cbe7385765d662d8ee28))


### Dependencies

* **actions:** bump github/codeql-action/upload-sarif ([#48](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/48)) ([a744791](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/a744791a81a322e2df6a9138bd7cfe9b760e82b4))
* **webapp:** bump the npm-minor-patch group across 1 directory with 7 updates ([#55](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/55)) ([ec6ad5b](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/ec6ad5b47d2bd754b2a73ce52a7bfbeea4bbb0b2))

## [0.7.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.6.0...v0.7.0) (2026-09-23)


### Features

* add markdown note links whose hover card renders tables, lists and checklists, and a /tactical-fusion note command ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* airfields gain IATA codes, runways, radio frequencies, a military designator and a numbered route map ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* decode FAA temporary flight restrictions and draw their circle or area in red on the report map ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* decode METAR, SPECI, TAF and NOTAM reports into plain language, as a link on one line and a card on several ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* decorate radio frequencies written behind FREQ: ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* **mcp:** decode aviation reports, Cursor on Target, GeoJSON, frequencies and date-time groups, and build CoT events and GeoJSON documents ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* **mcp:** expose four tools to the Agents plugin over MCP ([#51](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/51)) ([e7640d1](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/e7640d1e836aa99719300ceb8afeae5f97307585))


### Bug Fixes

* load note images through the image proxy when the server has one ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))
* require permission to post before /tactical-fusion note or examples posts for a user ([c257bce](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c257bce5d4bac8a1cea23de2e72de25e2bf1f82e))

## [0.6.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.5.1...v0.6.0) (2026-09-17)


### Features

* **map:** taller inline map, and a larger view that keeps its camera ([#45](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/45)) ([d1a361a](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/d1a361a953f26503a6b10278116d4f2383cb631d))

## [0.5.1](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.5.0...v0.5.1) (2026-09-16)


### Bug Fixes

* leave pasted USMTF messages undecorated ([#40](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/40)) ([8210c8e](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/8210c8ee6db954a17ba818f478a519aecbeafb6c))


### Dependencies

* **actions:** bump github/codeql-action/upload-sarif ([#36](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/36)) ([5d2915c](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/5d2915c7a2c664ba88fc8d42d6b9bc6d2c442ccf))
* **webapp:** bump the npm-minor-patch group across 1 directory with 14 updates ([#43](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/43)) ([81add5d](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/81add5db74aaf086b6ebb0690969037936aebf91))

## [0.5.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.4.0...v0.5.0) (2026-09-15)


### Features

* add a plugin bridge so other plugins can embed decorator links ([#38](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/38)) ([7178155](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/7178155770c20b597a4e773c84a3ab15e7b91a88))

## [0.4.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.3.0...v0.4.0) (2026-09-01)


### Features

* render GeoJSON documents, and open an overlay as a full-window map ([#30](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/30)) ([83a79cb](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/83a79cb2a20c670807b1f45c84171df33bd83762))


### Dependencies

* **actions:** bump the actions-minor-patch group across 1 directory with 3 updates ([#34](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/34)) ([7d469b4](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/7d469b4702e3577edcbb704ba1f5057d520f897e))
* **webapp:** bump the npm-minor-patch group across 1 directory with 4 updates ([#31](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/31)) ([2af6db5](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/2af6db5f34065fb4c2401c589380aa18f0967119))

## [0.3.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.2.0...v0.3.0) (2026-08-27)


### ⚠ BREAKING CHANGES

* **cot:** /tactical-fusion example-details no longer exists. Everything it posted is in the built-in help, on the page for each format.

### Features

* **cot:** render Cursor on Target events as a card, panel and map ([#27](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/27)) ([056c899](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/056c899178220e9be6e6b6adce1eec5a8e9af570))


### Bug Fixes

* **maps:** stamp the bundled Hawaii archive and correct the size inventory ([#22](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/22)) ([81a7b3f](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/81a7b3f86b4b562c8201417c26e650df63af747c))


### Dependencies

* **actions:** bump github/codeql-action/upload-sarif ([#25](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/25)) ([e451d2f](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/e451d2f206434323ef10c3ecb56b88c395dd1b30))
* **go:** bump github.com/mattermost/mattermost/server/public ([#24](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/24)) ([5deed93](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/5deed9379861a82a144fe1cbc3d0f81b0046cba6))
* **webapp:** bump the npm-minor-patch group across 1 directory with 6 updates ([#29](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/29)) ([4a2b74d](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/4a2b74d8c2b94214cc1ce7151289b20916339eb7))

## [0.2.0](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/compare/v0.1.0...v0.2.0) (2026-08-23)


### Features

* **airport:** recognize ICAO airfield codes behind USMTF field labels ([#19](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/19)) ([7c68176](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/7c68176744768ec8808284523f862233889b0f5e))
* **maps:** draw OpenStreetMap detail above the Natural Earth seam ([#21](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/21)) ([c2182f2](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/c2182f292b898e53e0bc1aae3c22e837353678ab))


### Bug Fixes

* **ci:** read the Node version from .nvmrc in PR validation too ([#16](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/16)) ([1df7b27](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/1df7b27e8e7379e15a50c84e1b42b82471c27da5))
* **security:** suppress GO-2026-5932, which no version bump can clear ([#18](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/18)) ([7b19aa3](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/7b19aa3a1909c67c4b6527ab7d33fdf53d9a545b))


### Dependencies

* bump golang.org/x/crypto to 0.55.0 to clear GO-2026-5932 ([#17](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/17)) ([37db5ee](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/37db5ee442b88cc634abefddf68950b13d4794e9))

## 0.1.0 (2026-08-18)


### Features

* add the location decorator ([#6](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/6)) ([ea1c7b6](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/ea1c7b6e4c65360d278808520123ec42f1affa17))
* decorate date-time groups and RFC 3339 timestamps in posted messages ([8f66053](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/8f66053575c59c310fec6399caa6040197a2a854))
* draw coordinates on a bundled offline map ([#13](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/13)) ([f6948e3](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/f6948e32ed4952d266c9bf9c149a0cbbf67c817d))
* **location:** add GEOREF, GARS and Plus Code area references ([#9](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/9)) ([d7ff44e](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/d7ff44eeb9402ebb39fe8f50543a732d9a58efed))


### Bug Fixes

* **release:** seed at 0.0.0 and keep major bumps out of commit messages ([#7](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/7)) ([25e3529](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/25e3529b25c6acaf4c5b74e1af2049e153177bc9))
* **release:** set initial-version so the first release is 0.1.0 ([#8](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/8)) ([84f7861](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/84f7861460f0682718bd75f0a86677c9b2728710))
* **webapp:** bump js-yaml to 4.3.1 for GHSA-5p4m-2wfm-xmqj ([#3](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/3)) ([45c1248](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/45c124896fed70ace6da9d97888c55aed33806d8))


### Dependencies

* **actions:** bump github/codeql-action/upload-sarif ([#12](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/12)) ([6827630](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/6827630f12e715a90dbf13d88e79ab33fa2e6a7c))
* **actions:** bump github/codeql-action/upload-sarif ([#2](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/2)) ([81f8598](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/81f8598e30727797460f61143752508467f08011))
* **webapp:** bump c8 from 11.0.0 to 12.0.0 in /webapp ([#11](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/11)) ([e86a2d4](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/e86a2d4c4cedfa7f7e99b922a48e3b15e18de8c9))
* **webapp:** bump the npm-minor-patch group across 1 directory with 3 updates ([#1](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/1)) ([861789a](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/861789aea912afa431ed41f5d026ad42f727b432))
* **webapp:** bump the npm-minor-patch group across 1 directory with 3 updates ([#10](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/issues/10)) ([2401496](https://github.com/MattermostFederal/mattermost-plugin-tactical-fusion/commit/240149693851893db4112ff0261754dc089ad713))

## Changelog

All notable changes to this project are documented in this file.

This file is maintained automatically by
[release-please](https://github.com/googleapis/release-please) from
[Conventional Commits](https://www.conventionalcommits.org/). Do not edit it by
hand. See [docs/RELEASING.md](docs/RELEASING.md).
