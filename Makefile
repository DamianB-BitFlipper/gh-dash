PIPEDIFFSHUB_DIR ?= tools/pipediffshub
DIFFVIEWER_DIST := internal/tui/diffviewer/dist
LOCAL_EXTENSION_DIR := .dehub-local-extension/gh-dehub

.PHONY: build build-pipediffshub build-dehub local-install

build: build-pipediffshub build-dehub

build-pipediffshub:
	cd $(PIPEDIFFSHUB_DIR) && bun install
	cd $(PIPEDIFFSHUB_DIR) && bun run build
	rm -rf $(DIFFVIEWER_DIST)
	mkdir -p internal/tui/diffviewer
	cp -R $(PIPEDIFFSHUB_DIR)/dist $(DIFFVIEWER_DIST)

build-dehub:
	go build -o gh-dehub .

local-install: build
	gh ext remove dehub 2>/dev/null || true
	rm -rf $(LOCAL_EXTENSION_DIR)
	mkdir -p $(LOCAL_EXTENSION_DIR)
	cp gh-dehub $(LOCAL_EXTENSION_DIR)/gh-dehub
	cd $(LOCAL_EXTENSION_DIR) && gh ext install .
