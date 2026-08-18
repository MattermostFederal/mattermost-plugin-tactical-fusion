# Changelog

## 0.1.0 (2026-08-18)


### ⚠ BREAKING CHANGES

* URLs that previously rendered without a Mattermost login now redirect to one. The argument for the route being public was that the mobile app's in-app browser has no session, and that was never verified in either direction; it is still open, and it now decides whether a decorated link on mobile costs a sign-in before the page. The "Unverified before deployment" section of CLAUDE.md says so. Static assets under /plugins/<id>/public/** are served by Mattermost rather than by this plugin and are unaffected.
* **release:** footer for the plugin id rename, and that is what drove it. Without bump-minor-pre-major a breaking footer takes an existing 0.x straight to 1.0.0; a plain feat: would have given 0.2.0. The fix is unchanged and still correct, but the reason given for it was not, so it is corrected everywhere it was stated.
* the plugin id changes from com.mattermost.plugin-mission-context to com.mattermost.plugin-tactical-fusion. Mattermost keys a plugin's settings and KV data by id, so any existing dev install is orphaned rather than upgraded, and decorator links already written into messages point at the old /plugins/<id>/decorate path and will 404. Both are acceptable only because this has never shipped.

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
