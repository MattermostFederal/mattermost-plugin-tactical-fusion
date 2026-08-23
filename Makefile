GO ?= $(shell command -v go 2> /dev/null)
NPM ?= $(shell command -v npm 2> /dev/null)
CURL ?= $(shell command -v curl 2> /dev/null)
MM_DEBUG ?=
GOPATH ?= $(shell go env GOPATH)
GO_TEST_FLAGS ?= -race
GO_BUILD_FLAGS ?=
DEFAULT_GOOS := $(shell go env GOOS)
DEFAULT_GOARCH := $(shell go env GOARCH)

export GO111MODULE=on

# We need to export GOBIN to allow it to be set
# for processes spawned from the Makefile
export GOBIN ?= $(PWD)/bin

# You can include assets this directory into the bundle. This can be e.g. used to include profile pictures.
ASSETS_DIR ?= assets

# Pinned so an archive rebuilt later is rebuilt by the same tiler.
TIPPECANOE_VERSION ?= 2.78.0

# The OpenStreetMap detail tier's generator, pinned for the same reason. The
# digest travels with the version: moving one without the other fails the build.
PLANETILER_OMT_VERSION ?= v3.16
PLANETILER_OMT_SHA256 ?= 246cd5c9c10102a3bcc58465ae7dde5b97aa4cee6524ea25788e23333ba2579d

# Verify environment, and define PLUGIN_ID, PLUGIN_VERSION, HAS_SERVER and HAS_WEBAPP as needed.
include build/setup.mk

BUNDLE_NAME ?= $(PLUGIN_ID)-$(PLUGIN_VERSION).tar.gz

# Include custom makefile, if present
ifneq ($(wildcard build/custom.mk),)
	include build/custom.mk
endif

ifneq ($(MM_DEBUG),)
	GO_BUILD_GCFLAGS = -gcflags "all=-N -l"
else
	GO_BUILD_GCFLAGS =
endif

# ====================================================================================
# Default Target
# ====================================================================================

.PHONY: default
default: all

# ====================================================================================
# Build Targets
# ====================================================================================

## Checks the code style, tests, builds and bundles the plugin.
.PHONY: all
all: check-style test dist

## Pre-release checks: git status and changelog validation.
.PHONY: release-check
release-check:
	@echo "Running pre-release checks..."
	@if [ -n "$$(git status --porcelain -- . ':!webapp/package-lock.json')" ]; then \
		echo "ERROR: Working directory has uncommitted changes."; \
		echo "Please commit or stash changes before building a release."; \
		git status --short -- . ':!webapp/package-lock.json'; \
		exit 1; \
	fi
	@if [ ! -f CHANGELOG.md ]; then \
		echo "ERROR: CHANGELOG.md not found."; \
		exit 1; \
	fi
	@if ! grep -q "## \[Unreleased\]" CHANGELOG.md && ! grep -q "## \[$(PLUGIN_VERSION)\]" CHANGELOG.md; then \
		echo "WARNING: CHANGELOG.md may not be updated for version $(PLUGIN_VERSION)."; \
	fi
	@echo "Pre-release checks passed."

## Generate SHA256 checksum for the release bundle.
.PHONY: release-checksum
release-checksum:
	@echo "Generating SHA256 checksum..."
	@cd dist && shasum -a 256 $(BUNDLE_NAME) > $(BUNDLE_NAME).sha256
	@echo "Checksum: $$(cat dist/$(BUNDLE_NAME).sha256)"

## Include SBOMs and CodeQL results in the release bundle and repackage.
.PHONY: release-bundle
release-bundle:
	@echo "Including SBOMs and security reports in release bundle..."
	@if [ -d dist/sbom ]; then \
		cp -r dist/sbom dist/$(PLUGIN_ID)/; \
		echo "SBOMs included in bundle"; \
	else \
		echo "WARNING: No SBOMs found to include"; \
	fi
	@mkdir -p dist/$(PLUGIN_ID)/security
	@if [ -f dist/codeql-go.sarif ]; then \
		cp dist/codeql-go.sarif dist/$(PLUGIN_ID)/security/; \
		echo "Go CodeQL results included"; \
	fi
	@if [ -f dist/codeql-js.sarif ]; then \
		cp dist/codeql-js.sarif dist/$(PLUGIN_ID)/security/; \
		echo "JavaScript CodeQL results included"; \
	fi
	@rm -f dist/$(BUNDLE_NAME)
	@if [ "$$(uname)" = "Darwin" ]; then \
		cd dist && tar --disable-copyfile -cvzf $(BUNDLE_NAME) $(PLUGIN_ID); \
	else \
		cd dist && tar -cvzf $(BUNDLE_NAME) $(PLUGIN_ID); \
	fi

## Sign the plugin bundle with GPG (requires PLUGIN_SIGNING_KEY env var).
.PHONY: release-sign
release-sign:
	@if [ -n "$(PLUGIN_SIGNING_KEY)" ]; then \
		echo "Signing plugin bundle with GPG key $(PLUGIN_SIGNING_KEY)..."; \
		gpg -u $(PLUGIN_SIGNING_KEY) --verbose --personal-digest-preferences SHA256 --detach-sign dist/$(BUNDLE_NAME); \
		echo "Signature: dist/$(BUNDLE_NAME).sig"; \
	else \
		echo "PLUGIN_SIGNING_KEY not set, skipping signing."; \
		echo "To sign, set PLUGIN_SIGNING_KEY to your GPG key ID."; \
	fi

## Create a git tag for the release version.
.PHONY: release-tag
release-tag:
	@echo "Creating git tag v$(PLUGIN_VERSION)..."
	@if git rev-parse "v$(PLUGIN_VERSION)" >/dev/null 2>&1; then \
		echo "ERROR: Tag v$(PLUGIN_VERSION) already exists."; \
		exit 1; \
	fi
	git tag -a "v$(PLUGIN_VERSION)" -m "Release v$(PLUGIN_VERSION)"
	@echo "Tag v$(PLUGIN_VERSION) created. Push with: git push origin v$(PLUGIN_VERSION)"

## Full release build: clean, checks, style, tests, build, SBOM audit, CodeQL, bundle with SBOMs, sign, and checksum.
.PHONY: release
release: release-check clean all sbom-audit codeql-analyze security-gate release-bundle virus-scan release-sign release-checksum
	@echo ""
	@echo "=========================================="
	@echo "Release build complete!"
	@echo "Bundle:   dist/$(BUNDLE_NAME)"
	@echo "Checksum: dist/$(BUNDLE_NAME).sha256"
	@if [ -f dist/$(BUNDLE_NAME).sig ]; then echo "Signature: dist/$(BUNDLE_NAME).sig"; fi
	@echo "SBOMs included in bundle"
	@echo ""
	@echo "To tag this release: make release-tag"
	@echo "=========================================="

## Ensures the plugin manifest is valid
.PHONY: manifest-check
manifest-check:
	./build/bin/manifest check

## Propagates plugin manifest information into the server/ and webapp/ folders.
.PHONY: apply
apply:
	./build/bin/manifest apply

## Rebuilds the vector basemap archive from Natural Earth.
##
## Runs tippecanoe in a container, so the toolchain is pinned rather than
## whatever a developer happens to have, and is a prerequisite of NOTHING: it is
## never reached by `make test` and never runs in CI. Commit the result.
.PHONY: map-tiles
map-tiles:
	docker build --build-arg TIPPECANOE_VERSION=$(TIPPECANOE_VERSION) \
		-t tf-maptiles:$(TIPPECANOE_VERSION) build/maptiles/
	docker run --rm -v "$(PWD)":/work tf-maptiles:$(TIPPECANOE_VERSION) build/maptiles/build.sh

## Rebuilds the OpenStreetMap detail tier, which takes over above the seam.
##
## Runs planetiler in a container, so the toolchain is pinned rather than whatever
## a developer happens to have, and is a prerequisite of NOTHING: it is never
## reached by `make test` and never runs in CI.
##
## PROFILE selects which regions in build/maposm/regions.txt are built. Only a
## row marked `bundled` writes into public/map/packages/ to be committed; every
## other row lands in build/maposm/out/ and ships as a release asset.
##
## JAVA_HEAP overrides the heap the input size would pick. See build/maposm/README.md.
##
## Rebuilds the offline detail map archives selected by PROFILE
.PHONY: map-osm
map-osm:
	docker build --build-arg TIPPECANOE_VERSION=$(TIPPECANOE_VERSION) \
		--build-arg PLANETILER_OMT_VERSION=$(PLANETILER_OMT_VERSION) \
		--build-arg PLANETILER_OMT_SHA256=$(PLANETILER_OMT_SHA256) \
		-t tf-maposm:$(PLANETILER_OMT_VERSION) build/maposm/
	docker run --rm -v "$(PWD)":/work -e PROFILE="$(PROFILE)" -e JAVA_HEAP="$(JAVA_HEAP)" \
		-e ALLOW_MIXED_DATES="$(ALLOW_MIXED_DATES)" \
		tf-maposm:$(PLANETILER_OMT_VERSION) build/maposm/build.sh

## Attaches the release-asset map packages in build/maposm/out/ to an existing
## GitHub release, with a checksum file beside them.
##
## Manual and post-release by design: the release workflow can neither download
## the ~10 GB of extracts nor spend the hours of tiling these need, so the
## archives are built on a workstation and uploaded afterwards.
##
##   make map-publish TAG=v0.3.0
##
## Uploads the built map packages to an existing GitHub release
.PHONY: map-publish
map-publish:
ifndef TAG
	$(error TAG is required, e.g. make map-publish TAG=v0.3.0)
endif
	@ls build/maposm/out/*.pmtiles >/dev/null 2>&1 || { \
		echo "error: no archives in build/maposm/out/; run 'make map-osm PROFILE=<name>' first."; \
		exit 1; \
	}
	@cd build/maposm/out && shasum -a 256 *.pmtiles > PACKAGES.sha256
	@echo "Uploading to $(TAG):"
	@ls -l build/maposm/out/
	gh release upload "$(TAG)" build/maposm/out/*.pmtiles build/maposm/out/PACKAGES.sha256 --clobber

## Downloads the OpenStreetMap extracts the detail tier is cut from and verifies
## them against build/maposm/sources.lock.
##
## PROFILE scopes it to one region or theatre, which matters because the full
## roster is about 10 GB. UPDATE_LOCK=1 re-pins what is in scope and leaves the
## rest of the lock alone:
##
##   UPDATE_LOCK=1 make osm-sources PROFILE=korea
##
## PIN_DATE pins to one Geofabrik cut instead of following -latest, which is how
## a set is held to a single snapshot when upstream is mid-rollover:
##
##   PIN_DATE=260820 UPDATE_LOCK=1 make osm-sources PROFILE=centcom
##
## Downloads the OpenStreetMap extracts and verifies them against the lock
.PHONY: osm-sources
osm-sources:
	PROFILE="$(PROFILE)" PIN_DATE="$(PIN_DATE)" ./build/maposm/fetch-sources.sh

## Downloads the 50m and 10m Natural Earth sources the finer tiers need and
## verifies them against build/maptiles/sources.lock. The 110m tier needs no
## download: those files are already committed for the country lookup.
.PHONY: map-sources
map-sources:
	./build/maptiles/fetch-sources.sh

## Regenerates the bundled basemap from the Natural Earth source in build/mapdata/source.
## The outputs are committed, so a clean checkout builds and an air-gapped `go test` runs
## without this target. Run it only when the source or the generator changes.
.PHONY: map-data
map-data:
	$(GO) run ./build/mapdata

## Fails when the committed country polygons are not what the committed source produces
## through the current generator. Idempotence is the wrong check: a generator edited without
## regenerating is perfectly idempotent and fully drifted.
.PHONY: map-data-check
map-data-check: map-data
	@# Named individually rather than by directory: mapdata.go sits beside admin.go
	@# and is hand-written, and watching the whole directory reported an ordinary
	@# edit to the decoder as stale map data and told the reader to run a generator
	@# that would not fix it.
	@#
	@# The basemap archive is deliberately absent from this list. It is built by
	@# make map-tiles, which needs a container and a toolchain, and this target is a
	@# prerequisite of make test.
	@git diff --exit-code -- server/decorators/location/mapdata/admin.go \
		|| (echo "map data is stale: run 'make map-data' and commit the result" && exit 1)

## Regenerates the embedded airfield database from an upstream airport-codes.csv.
## Deliberately NOT a prerequisite of test, unlike map-data-check: this transform is
## filter, round and drop columns, and its drift means an airfield code declines, which
## is visible and harmless. map-data-check earns that slot because its encoding is opaque
## and its drift fails invisibly on a plain-HTTP origin. The upstream file is not
## committed; see server/decorators/airport/data/README.md for where to put it.
.PHONY: airport-data
airport-data:
	$(GO) run ./build/airportdata

## Builds the server, if it exists, for all supported architectures, unless MM_SERVICESETTINGS_ENABLEDEVELOPER is set.
.PHONY: server
server:
ifneq ($(HAS_SERVER),)
ifneq ($(MM_DEBUG),)
	$(info DEBUG mode is on; to disable, unset MM_DEBUG)
endif
	mkdir -p server/dist;
ifneq ($(MM_SERVICESETTINGS_ENABLEDEVELOPER),)
	@echo Building plugin only for $(DEFAULT_GOOS)-$(DEFAULT_GOARCH) because MM_SERVICESETTINGS_ENABLEDEVELOPER is enabled
	cd server && env CGO_ENABLED=0 $(GO) build $(GO_BUILD_FLAGS) $(GO_BUILD_GCFLAGS) -trimpath -o dist/plugin-$(DEFAULT_GOOS)-$(DEFAULT_GOARCH);
else
	cd server && env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GO_BUILD_FLAGS) $(GO_BUILD_GCFLAGS) -trimpath -o dist/plugin-linux-amd64;
	cd server && env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GO_BUILD_FLAGS) $(GO_BUILD_GCFLAGS) -trimpath -o dist/plugin-linux-arm64;
endif
endif

## Ensures NPM dependencies are installed without having to run this all the time.
webapp/node_modules: $(wildcard webapp/package.json)
ifneq ($(HAS_WEBAPP),)
	cd webapp && $(NPM) install
	touch $@
endif

## Builds the webapp, if it exists.
.PHONY: webapp
webapp: webapp/node_modules
ifneq ($(HAS_WEBAPP),)
ifeq ($(MM_DEBUG),)
	cd webapp && $(NPM) run build;
else
	cd webapp && $(NPM) run debug;
endif
endif

## Generates a tar bundle of the plugin for install.
.PHONY: bundle
bundle:
	rm -rf dist/
	mkdir -p dist/$(PLUGIN_ID)
	./build/bin/manifest dist
ifneq ($(wildcard $(ASSETS_DIR)/.),)
	cp -r $(ASSETS_DIR) dist/$(PLUGIN_ID)/
endif
ifneq ($(HAS_PUBLIC),)
	cp -r public dist/$(PLUGIN_ID)/
	@# The map ships as three things that fail separately and all fail quietly,
	@# which is why they are checked here, where the bundle is actually
	@# assembled, rather than by a test against the working tree.
	@#
	@# `cp -r public` copies whatever is on disk with no exclusion, and neither
	@# `make clean` nor `make nuke` touches public/, so what ships is whatever a
	@# developer last left there.
	@if [ ! -f "dist/$(PLUGIN_ID)/public/map/world.pmtiles" ]; then \
		echo "ERROR: the bundle is missing public/map/world.pmtiles."; \
		echo "Every map surface draws from it. Run 'make map-tiles'."; \
		exit 1; \
	fi
	@# An LFS pointer, a truncated copy, or an HTML error page saved over the
	@# archive all leave a file of plausible size that MapLibre cannot read. The
	@# magic bytes are the cheapest thing that tells them apart.
	@if [ "$$(head -c 7 dist/$(PLUGIN_ID)/public/map/world.pmtiles)" != "PMTiles" ]; then \
		echo "ERROR: public/map/world.pmtiles is not a PMTiles archive."; \
		echo "Check it is not an LFS pointer or a truncated copy."; \
		exit 1; \
	fi
	@# The style names these by URL. Without them the country labels have no
	@# typeface to be drawn in, and the only symptom is a map that quietly
	@# stopped naming anything.
	@# Every range, not just the first: a missing 256-511 costs every accented
	@# name and nothing else would fail.
	@for r in 0-255 256-511 512-767 7680-7935 8192-8447; do \
		if [ ! -f "dist/$(PLUGIN_ID)/public/map/fonts/NotoSans-Regular/$$r.pbf" ]; then \
			echo "ERROR: the bundle is missing glyph range $$r under public/map/fonts."; \
			echo "The map's labels need them. Run 'make map-tiles'."; \
			exit 1; \
		fi; \
	done
	@# SIL OFL 1.1 requires the notice to travel with the font software, and SDF
	@# ranges generated from a TTF are a Modified Version of it. Shipping the
	@# fonts without this file is a licence violation, not an untidy bundle.
	@if [ ! -f "dist/$(PLUGIN_ID)/public/map/fonts/LICENSE.txt" ]; then \
		echo "ERROR: the glyph ranges ship without public/map/fonts/LICENSE.txt."; \
		echo "SIL OFL 1.1 requires the notice to travel with them."; \
		exit 1; \
	fi
	@# The OpenStreetMap detail tier is OPTIONAL: a global-only build is a
	@# supported profile, so its absence is not an error. What is an error is
	@# shipping it without the notice, since ODbL requires it to travel with the
	@# data, and shipping something under that name that is not an archive.
	@for pkg in dist/$(PLUGIN_ID)/public/map/packages/*.pmtiles; do \
		[ -e "$$pkg" ] || continue; \
		if [ "$$(head -c 7 "$$pkg")" != "PMTiles" ]; then \
			echo "ERROR: $$pkg is not a PMTiles archive."; \
			echo "Check it is not an LFS pointer or a truncated copy."; \
			exit 1; \
		fi; \
		case "$$(basename "$$pkg" .pmtiles)" in \
			*[!a-z0-9-]*|-*|*-|*--*) \
				echo "ERROR: $$pkg is not named <command>-<area>.pmtiles."; \
				echo "The name reaches a URL, so nothing will serve it."; \
				exit 1 ;; \
			*-*) ;; \
			*) \
				echo "ERROR: $$pkg is not named <command>-<area>.pmtiles."; \
				echo "The name reaches a URL, so nothing will serve it."; \
				exit 1 ;; \
		esac; \
		if [ ! -f "dist/$(PLUGIN_ID)/public/map/LICENSE-OSM.txt" ]; then \
			echo "ERROR: a detail package ships without public/map/LICENSE-OSM.txt."; \
			echo "ODbL 1.0 requires the notice to travel with the data."; \
			exit 1; \
		fi; \
	done
endif
ifneq ($(HAS_SERVER),)
	mkdir -p dist/$(PLUGIN_ID)/server
	cp -r server/dist dist/$(PLUGIN_ID)/server/
endif
ifneq ($(HAS_WEBAPP),)
	mkdir -p dist/$(PLUGIN_ID)/webapp
	cp -r webapp/dist dist/$(PLUGIN_ID)/webapp/
	@# The standalone pages are rendered by webpack's second bundle. Without it
	@# the pages serve their shell and render nothing at all.
	@if [ ! -f "dist/$(PLUGIN_ID)/public/app/page.js" ]; then \
		echo "ERROR: the bundle is missing public/app/page.js."; \
		echo "The standalone pages render nothing without it. Run 'make dist'."; \
		exit 1; \
	fi
	@# MapLibre's worker is a module that imports ./maplibre-gl-shared.mjs by that
	@# literal name. Shipping the worker without it is silent: the worker 404s,
	@# the style never finishes, and the map sits on "Loading map..." with no
	@# error. Neither the type checker nor any test sees this, so it is checked
	@# here, where the bundle is actually assembled.
	@if ls dist/$(PLUGIN_ID)/webapp/dist/maplibre-gl-worker.*.mjs >/dev/null 2>&1 && \
		[ ! -f dist/$(PLUGIN_ID)/webapp/dist/maplibre-gl-shared.mjs ]; then \
		echo "ERROR: the MapLibre worker ships without maplibre-gl-shared.mjs beside it."; \
		echo "The worker imports it by a fixed relative name and will 404 at runtime."; \
		exit 1; \
	fi
endif
ifeq ($(shell uname),Darwin)
	cd dist && tar --disable-copyfile -cvzf $(BUNDLE_NAME) $(PLUGIN_ID)
else
	cd dist && tar -cvzf $(BUNDLE_NAME) $(PLUGIN_ID)
endif

	@echo plugin built at: dist/$(BUNDLE_NAME)

## Builds and bundles the plugin.
.PHONY: dist
dist: apply server webapp bundle

# ====================================================================================
# Quality Targets
# ====================================================================================

## Install go tools
install-go-tools:
	@echo Installing go tools
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0
	$(GO) install gotest.tools/gotestsum@v1.13.0

## Runs eslint and golangci-lint
.PHONY: check-style
check-style: manifest-check apply webapp/node_modules install-go-tools
	@echo Checking for style guide compliance

ifneq ($(HAS_WEBAPP),)
	cd webapp && npm run lint
	cd webapp && npm run check-types
endif

# It's highly recommended to run go-vet first
# to find potential compile errors that could introduce
# weird reports at golangci-lint step
ifneq ($(HAS_SERVER),)
	@echo Running golangci-lint
	$(GO) vet ./server/...
	$(GOBIN)/golangci-lint run ./server/...
endif

## Runs any lints and unit tests defined for the server and webapp, if they exist.
.PHONY: test
test: map-data-check apply webapp/node_modules install-go-tools
ifneq ($(HAS_SERVER),)
	$(GOBIN)/gotestsum -- -v ./...
endif
ifneq ($(HAS_WEBAPP),)
	cd webapp && $(NPM) run test;
endif

## Runs any lints and unit tests defined for the server and webapp, if they exist, optimized for a CI environment.
.PHONY: test-ci
test-ci: apply webapp/node_modules install-go-tools
ifneq ($(HAS_SERVER),)
	$(GOBIN)/gotestsum --format standard-verbose --junitfile report.xml -- ./...
endif
ifneq ($(HAS_WEBAPP),)
	cd webapp && $(NPM) run test;
endif

## Prints Go code coverage summary to terminal.
.PHONY: coverage-backend
coverage-backend: apply
ifneq ($(HAS_SERVER),)
# -coverpkg is load-bearing. Without it each package is measured only by its own
# tests, so every call across a package boundary is invisible: the shared page
# shell in server/decorators reads as 0% while being fully exercised by the
# tests in server. That under-reports the total and, worse, points anybody
# reading this output at the wrong files.
	$(GO) test $(GO_TEST_FLAGS) -short -coverpkg=./server/... -coverprofile=server/coverage.txt ./server/...
	$(GO) tool cover -func=server/coverage.txt
endif

## Prints frontend code coverage summary to terminal.
.PHONY: coverage-frontend
coverage-frontend: webapp/node_modules
ifneq ($(HAS_WEBAPP),)
	cd webapp && $(NPM) run test:coverage
	cd webapp && $(NPM) run test:pw-ct-coverage
	@echo ""
	@echo "=== Merged Coverage (unit + component) ==="
	cd webapp && $(NPM) run test:coverage-merged
endif

## Prints code coverage summary for both backend and frontend.
.PHONY: coverage
coverage: coverage-backend coverage-frontend

## Clean removes all build artifacts (but preserves build tools).
.PHONY: clean
clean:
	rm -fr dist/
ifneq ($(HAS_SERVER),)
	rm -fr server/coverage.txt
	rm -fr server/dist
endif
ifneq ($(HAS_WEBAPP),)
	rm -fr webapp/junit.xml
	rm -fr webapp/dist
	rm -fr webapp/node_modules
	rm -fr webapp/coverage
	rm -fr webapp/coverage-ct
	rm -fr webapp/coverage-merged
	rm -fr webapp/.v8-ct-coverage
	rm -fr webapp/.v8-unit-coverage
	rm -fr webapp/.v8-merged-coverage
endif

## Nuke everything: Docker containers, data, and all build artifacts
.PHONY: nuke
nuke: docker-kill-orphans
	@echo "Nuking everything..."
	@$(DOCKER_COMPOSE) down -v 2>/dev/null || true
	@rm -rf docker/postgres-data docker/mattermost
	@rm -fr dist/
	@rm -fr server/coverage.txt server/dist
	@rm -fr webapp/junit.xml webapp/dist webapp/node_modules
	@rm -fr webapp/coverage webapp/coverage-ct webapp/coverage-merged
	@rm -fr webapp/.v8-ct-coverage webapp/.v8-unit-coverage webapp/.v8-merged-coverage
	@rm -fr build/bin/
	@echo "Everything removed. Run 'make docker-setup' to start fresh."

## Generate mocks
.PHONY: mock
mock:
ifneq ($(HAS_SERVER),)
	go install go.uber.org/mock/mockgen@v0.6.0
	@echo "No mocks configured for this plugin. Add your mockgen commands here."
endif

# ====================================================================================
# Docker Development Environment
# ====================================================================================
DOCKER_COMPOSE := docker compose -f docker-compose.dev.yml
MM_PORT ?= 8065

MAP_PACKAGE_DIR ?= map-packages
MAP_PACKAGE_HOST := docker/mattermost/data/$(MAP_PACKAGE_DIR)
MAP_PACKAGE_PATH := /mattermost/data/$(MAP_PACKAGE_DIR)

## Start Mattermost and PostgreSQL containers
.PHONY: docker-start
docker-start:
	@echo "Starting Mattermost Enterprise Edition..."
	@mkdir -p docker/mattermost/{config,data,logs,plugins,client-plugins}
	@mkdir -p docker/postgres-data
	@$(DOCKER_COMPOSE) up -d

## Stop containers (preserves data)
.PHONY: docker-stop
docker-stop:
	@$(DOCKER_COMPOSE) stop

## Stop and remove containers
.PHONY: docker-down
docker-down:
	@$(DOCKER_COMPOSE) down

## Remove containers and all data
.PHONY: docker-clean
docker-clean:
	@$(DOCKER_COMPOSE) down -v
	@rm -rf docker/postgres-data docker/mattermost
	@echo "Containers and data removed"

## Kill orphaned Docker containers on the MM port (useful after deleting a worktree)
.PHONY: docker-kill-orphans
docker-kill-orphans:
	@project=$$(docker ps --filter "publish=$(MM_PORT)" \
		--format '{{.Label "com.docker.compose.project"}}' | head -1); \
	if [ -z "$$project" ]; then \
		echo "No containers found on port $(MM_PORT)"; \
	else \
		echo "Stopping compose project: $$project"; \
		docker compose -p $$project down -v; \
		echo "Project $$project removed"; \
	fi

## View Mattermost container logs
.PHONY: docker-logs
docker-logs: docker-check
	@$(DOCKER_COMPOSE) logs -f mattermost

## First-time setup: start containers and create admin user
.PHONY: docker-setup
docker-setup: docker-start
	@echo "Waiting for Mattermost to be ready..."
	@until curl -sf http://localhost:$(MM_PORT)/api/v4/system/ping >/dev/null 2>&1; do \
		sleep 2; \
		echo "Waiting..."; \
	done
	@echo "Creating admin user..."
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local user create \
		--email admin@example.com \
		--username admin \
		--password 'password' \
		--system-admin 2>/dev/null || echo "Admin user already exists"
	@echo "Creating default team..."
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local team create \
		--name test \
		--display-name "Test" 2>/dev/null || echo "Team 'Test' already exists"
	@echo "Adding admin to Test team..."
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local team users add test admin 2>/dev/null || echo "Admin already in Test team"
	@echo ""
	@echo "=========================================="
	@echo "Mattermost ready at http://localhost:$(MM_PORT)"
	@echo "Login: admin / password"
	@echo "Team: Test"
	@echo "=========================================="

## Check if Mattermost container is running
.PHONY: docker-check
docker-check:
	@if ! $(DOCKER_COMPOSE) ps --status running 2>/dev/null | grep -q mattermost; then \
		echo "Error: Mattermost container is not running."; \
		echo "Run 'make docker-setup' first to start the environment."; \
		exit 1; \
	fi

## Build and deploy plugin to Docker Mattermost
.PHONY: docker-deploy
docker-deploy: docker-check dist
	@echo "Deploying plugin to Docker Mattermost..."
	@$(DOCKER_COMPOSE) cp dist/$(BUNDLE_NAME) mattermost:/tmp/$(BUNDLE_NAME)
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin add /tmp/$(BUNDLE_NAME) --force
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin enable $(PLUGIN_ID)
	@echo "Plugin $(PLUGIN_ID) deployed and enabled"

## Disable and re-enable plugin in Docker
.PHONY: docker-reset
docker-reset: docker-check
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin disable $(PLUGIN_ID)
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin enable $(PLUGIN_ID)
	@echo "Plugin $(PLUGIN_ID) reset"

## Disable plugin in Docker
.PHONY: docker-disable
docker-disable: docker-check
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin disable $(PLUGIN_ID)

## Enable plugin in Docker
.PHONY: docker-enable
docker-enable: docker-check
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin enable $(PLUGIN_ID)

## List installed plugins in Docker
.PHONY: docker-plugin-list
docker-plugin-list: docker-check
	@$(DOCKER_COMPOSE) exec -T mattermost mmctl --local plugin list

## Copies the release-asset map packages into the Docker server's drop-in
## directory and points LocationMapPackagesDir at it, so a development install
## draws every area without downloading anything.
##
## Bundled regions are already inside the plugin and are deliberately not copied:
## a drop-in copy would shadow the bundled one and stop exercising the bundled
## path. Only files whose source is newer are copied, so a redeploy is cheap.
##
## Drops every built map area into the Docker server for testing
.PHONY: docker-packages
docker-packages: docker-check
	@if ! ls build/maposm/out/*.pmtiles >/dev/null 2>&1; then \
		echo "No archives in build/maposm/out/. Build some with 'make map-osm PROFILE=<name>'."; \
	else \
		mkdir -p $(MAP_PACKAGE_HOST); \
		n=0; \
		for f in build/maposm/out/*.pmtiles; do \
			d="$(MAP_PACKAGE_HOST)/$$(basename $$f)"; \
			if [ ! -f "$$d" ] || [ "$$f" -nt "$$d" ]; then \
				cp "$$f" "$$d"; \
				echo "  copied $$(basename $$f)"; \
				n=$$((n + 1)); \
			fi; \
		done; \
		if [ "$$n" -eq 0 ]; then echo "  packages already current"; fi; \
		printf '{"PluginSettings":{"Plugins":{"%s":{"locationmappackagesdir":"%s"}}}}' \
			"$(PLUGIN_ID)" "$(MAP_PACKAGE_PATH)" > docker/mattermost/data/.mappkg-patch.json; \
		$(DOCKER_COMPOSE) exec -T mattermost mmctl --local config patch \
			/mattermost/data/.mappkg-patch.json > /dev/null; \
		rm -f docker/mattermost/data/.mappkg-patch.json; \
		echo "LocationMapPackagesDir = $(MAP_PACKAGE_PATH), $$(ls $(MAP_PACKAGE_HOST)/*.pmtiles 2>/dev/null | wc -l | tr -d ' ') dropped-in areas"; \
	fi

## Deploys the plugin to Docker and drops in every built map area
.PHONY: deploy
deploy: docker-deploy docker-packages

## Build and deploy to a Mattermost server running at MM_LOCAL_SITEURL
## (default http://localhost:8065) via the bundled pluginctl tool. Unlike
## `make deploy` (which targets the docker-compose stack), this hits a
## locally-running server directly - useful when you develop against your own
## Mattermost rather than the bundled Docker environment.
##
## pluginctl authenticates via one of (it validates and picks natively):
##   - Local mode (auto-detected default socket, or MM_LOCALSOCKETPATH), or
##   - MM_ADMIN_TOKEN                          (an admin personal access token), or
##   - MM_ADMIN_USERNAME + MM_ADMIN_PASSWORD   (admin login)
## Override the target server with `make deploy-local MM_LOCAL_SITEURL=...`.
MM_LOCAL_SITEURL ?= http://localhost:8065
.PHONY: deploy-local
deploy-local: dist
	@MM_SERVICESETTINGS_SITEURL=$(MM_LOCAL_SITEURL) ./build/bin/pluginctl deploy $(PLUGIN_ID) dist/$(BUNDLE_NAME) || { \
		status=$$?; \
		echo "deploy-local failed. pluginctl authenticates via local mode (default socket or MM_LOCALSOCKETPATH), MM_ADMIN_TOKEN, or MM_ADMIN_USERNAME + MM_ADMIN_PASSWORD."; \
		echo "Or, with an already-authenticated mmctl, install directly:"; \
		echo "  mmctl plugin add dist/$(BUNDLE_NAME) --force && mmctl plugin enable $(PLUGIN_ID)"; \
		exit $$status; \
	}

# ====================================================================================
# SBOM & Vulnerability Scanning
# ====================================================================================

## Install SBOM generation tools
.PHONY: install-sbom-tools
install-sbom-tools:
	@echo "Installing SBOM generation tools..."
	$(GO) install github.com/CycloneDX/cyclonedx-gomod/cmd/cyclonedx-gomod@latest

## Install Grype vulnerability scanner
.PHONY: install-grype
install-grype:
	@if [ ! -x "$(GOBIN)/grype" ]; then \
		echo "Installing Grype via go install (cross-platform, no anchore install.sh)..."; \
		mkdir -p $(GOBIN); \
		GOBIN=$(GOBIN) $(GO) install github.com/anchore/grype/cmd/grype@latest; \
	else \
		echo "Grype already installed"; \
	fi

## Generate Software Bill of Materials (SBOM) in CycloneDX JSON format
.PHONY: sbom
sbom: install-sbom-tools
	@mkdir -p dist/sbom
ifneq ($(HAS_SERVER),)
	@echo "Generating Go SBOM..."
	$(GOBIN)/cyclonedx-gomod mod -json -output dist/sbom/server-sbom.json
endif
ifneq ($(HAS_WEBAPP),)
	@echo "Generating Node.js SBOM..."
	cd webapp && npx @cyclonedx/cyclonedx-npm --ignore-npm-errors --output-file ../dist/sbom/webapp-sbom.json
endif
	@echo "SBOMs generated in dist/sbom/"
	@ls -la dist/sbom/

## Scan SBOMs for vulnerabilities using Grype (fails on high or critical)
.PHONY: sbom-scan
sbom-scan: install-grype
	@if [ ! -d dist/sbom ]; then \
		echo "No SBOMs found. Run 'make sbom' first."; \
		exit 1; \
	fi
ifneq ($(HAS_SERVER),)
	@echo "Scanning Go dependencies for vulnerabilities..."
	$(GOBIN)/grype sbom:dist/sbom/server-sbom.json --output table --fail-on high
endif
ifneq ($(HAS_WEBAPP),)
	@echo "Scanning Node.js dependencies for vulnerabilities..."
	$(GOBIN)/grype sbom:dist/sbom/webapp-sbom.json --output table --fail-on high
endif

## Generate SBOMs and scan for vulnerabilities
.PHONY: sbom-audit
sbom-audit: sbom sbom-scan

# ====================================================================================
# CodeQL Security Analysis
# ====================================================================================

CODEQL_VERSION ?= 2.20.1
CODEQL_DIR := $(PWD)/build/codeql
CODEQL := $(CODEQL_DIR)/codeql/codeql
CODEQL_DB_DIR := $(PWD)/build/codeql-db

## Install CodeQL CLI
.PHONY: install-codeql
install-codeql:
	@if [ ! -f "$(CODEQL)" ]; then \
		echo "Installing CodeQL CLI v$(CODEQL_VERSION)..."; \
		mkdir -p $(CODEQL_DIR); \
		if [ "$$(uname)" = "Darwin" ]; then \
			CODEQL_PLATFORM="osx64"; \
		else \
			CODEQL_PLATFORM="linux64"; \
		fi; \
		curl -sSL "https://github.com/github/codeql-action/releases/download/codeql-bundle-v$(CODEQL_VERSION)/codeql-bundle-$$CODEQL_PLATFORM.tar.gz" | tar -xz -C $(CODEQL_DIR); \
		echo "CodeQL CLI installed"; \
	else \
		echo "CodeQL CLI already installed"; \
	fi

## Run CodeQL analysis on Go code
.PHONY: codeql-go
codeql-go: install-codeql
ifneq ($(HAS_SERVER),)
	@echo "Running CodeQL analysis on Go code..."
	@rm -rf $(CODEQL_DB_DIR)/go
	@mkdir -p $(CODEQL_DB_DIR)/go
	@mkdir -p dist
	$(CODEQL) database create $(CODEQL_DB_DIR)/go --language=go --source-root=server --overwrite
	$(CODEQL) database analyze $(CODEQL_DB_DIR)/go --format=sarif-latest --output=dist/codeql-go.sarif -- codeql/go-queries
	@echo "Go CodeQL results: dist/codeql-go.sarif"
endif

## Run CodeQL analysis on JavaScript/TypeScript code
.PHONY: codeql-js
codeql-js: install-codeql webapp/node_modules
ifneq ($(HAS_WEBAPP),)
	@echo "Running CodeQL analysis on JavaScript/TypeScript code..."
	@rm -rf $(CODEQL_DB_DIR)/js
	@mkdir -p $(CODEQL_DB_DIR)/js
	@mkdir -p dist
	$(CODEQL) database create $(CODEQL_DB_DIR)/js --language=javascript --source-root=webapp --overwrite
	$(CODEQL) database analyze $(CODEQL_DB_DIR)/js --format=sarif-latest --output=dist/codeql-js.sarif -- codeql/javascript-queries
	@echo "JavaScript/TypeScript CodeQL results: dist/codeql-js.sarif"
endif

## Run CodeQL analysis on all code
.PHONY: codeql-analyze
codeql-analyze: codeql-go codeql-js
	@echo "CodeQL analysis complete. Results in dist/codeql-*.sarif"

## Check CodeQL SARIF reports for critical/high severity issues (level=error in SARIF)
.PHONY: security-gate
security-gate:
	@echo "Checking security scan results for critical/high issues..."
	@failed=0; \
	for sarif in dist/codeql-*.sarif; do \
		[ -f "$$sarif" ] || continue; \
		count=$$(python3 -c "import json,sys;data=json.load(open(sys.argv[1]));print(sum(1 for run in data.get('runs',[]) for result in run.get('results',[]) if result.get('level')=='error'))" "$$sarif"); \
		if [ "$$count" -gt 0 ]; then \
			echo "ERROR: $$sarif contains $$count critical/high severity issue(s)."; \
			failed=1; \
		else \
			echo "OK: $$sarif has no critical/high severity issues."; \
		fi; \
	done; \
	if [ "$$failed" -eq 1 ]; then \
		echo ""; \
		echo "Security gate FAILED: Critical or high severity issues found."; \
		echo "Review the SARIF files in dist/ for details."; \
		exit 1; \
	fi
	@echo "Security gate passed."

# ====================================================================================
# Virus Scanning
# ====================================================================================

## Install ClamAV antivirus scanner
.PHONY: install-clamav
install-clamav:
	@if ! command -v clamscan >/dev/null 2>&1; then \
		echo "Installing ClamAV..."; \
		if [ "$$(uname)" = "Darwin" ]; then \
			brew install clamav; \
		else \
			sudo apt-get update && sudo apt-get install -y clamav; \
		fi; \
	else \
		echo "ClamAV already installed"; \
	fi
	@echo "Updating virus definitions..."
	@if [ "$$(uname)" = "Darwin" ]; then \
		if [ ! -f /opt/homebrew/etc/clamav/freshclam.conf ] && [ -f /opt/homebrew/etc/clamav/freshclam.conf.sample ]; then \
			cp /opt/homebrew/etc/clamav/freshclam.conf.sample /opt/homebrew/etc/clamav/freshclam.conf; \
			sed -i '' 's/^Example/#Example/' /opt/homebrew/etc/clamav/freshclam.conf; \
		fi; \
	else \
		sudo systemctl stop clamav-freshclam 2>/dev/null || true; \
	fi
	@sudo freshclam || freshclam

## Scan dist/ for viruses using ClamAV (fails if any detected)
.PHONY: virus-scan
virus-scan: install-clamav
	@if [ ! -d dist ]; then \
		echo "No dist/ directory found. Run 'make dist' first."; \
		exit 1; \
	fi
	@echo "Scanning release artifacts for viruses..."
	clamscan --recursive --infected --alert-broken dist/
	@echo "Virus scan passed."

# ====================================================================================
# Help
# ====================================================================================

help:
	@cat Makefile build/*.mk | grep -v '\.PHONY' |  grep -v '\help:' | grep -B1 -E '^[a-zA-Z0-9_.-]+:.*' | sed -e "s/:.*//" | sed -e "s/^## //" |  grep -v '\-\-' | sed '1!G;h;$$!d' | awk 'NR%2{printf "\033[36m%-30s\033[0m",$$0;next;}1' | sort
