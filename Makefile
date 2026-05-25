PIPEDIFFSHUB_DIR ?= tools/pipediffshub
DIFFVIEWER_DIST := internal/tui/diffviewer/dist

.PHONY: build build-pipediffshub build-gh-dash local-install

build: build-pipediffshub build-gh-dash

build-pipediffshub:
	cd $(PIPEDIFFSHUB_DIR) && bun install
	cd $(PIPEDIFFSHUB_DIR) && bun run build
	rm -rf $(DIFFVIEWER_DIST)
	mkdir -p internal/tui/diffviewer
	cp -R $(PIPEDIFFSHUB_DIR)/dist $(DIFFVIEWER_DIST)

build-gh-dash:
	go build .

local-install: build
	gh ext remove dash && gh ext install .
