# Makefile for kush — local build and Homebrew formula helpers
# Targets:
#  - build: build local binary for current platform
#  - build-cross: build for linux/amd64 linux/arm64 freebsd/amd64 freebsd/arm64 into dist/
#  - dist: create tar.gz of built binary for given OS/ARCH (set OS/ARCH env)
#  - goreleaser-local-mac: run goreleaser locally using the macOS config (.goreleaser.yml)
#  - brew-formula: emit a Homebrew formula file into dist/kush.rb
#  - release-draft: create a GitHub release draft and upload dist artifacts (requires gh cli and auth)

BINARY=kush
DIST_DIR=dist
VERSION?=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

.PHONY: all build build-cross dist clean goreleaser-local-mac brew-formula release-draft
all: build

build:
	go build -o $(BINARY) ./cmd/kush

build-cross:
	@mkdir -p $(DIST_DIR)
	set -e; \
	for GOOS in linux freebsd; do \
		for GOARCH in amd64 arm64; do \
			echo "Building $$GOOS/$$GOARCH..."; \
			BIN=$(DIST_DIR)/$(BINARY)-$$GOOS-$$GOARCH; \
			env GOOS=$$GOOS GOARCH=$$GOARCH go build -o $$BIN ./cmd/kush || exit 1; \
			tar -C $(DIST_DIR) -czf $(DIST_DIR)/$$(basename $$BIN).tar.gz $$(basename $$BIN); \
		done; \
	done

# dist target builds cross artifacts into $(DIST_DIR)
dist: build-cross

clean:
	@rm -rf $(BINARY) $(DIST_DIR)

# macOS goreleaser run locally — uses the macOS config (.goreleaser.yml)
# Remove existing dist/ before invoking goreleaser to ensure a clean artifact set.
goreleaser-local-mac:
	@rm -rf $(DIST_DIR)
	@goreleaser release --snapshot --rm-dist --config .goreleaser.yml

# Emit a Homebrew Cask file (Homebrew Casks system). This prints to dist/kush_cask.rb
brew-formula:
	@mkdir -p $(DIST_DIR)
	@cat > $(DIST_DIR)/kush_cask.rb <<'EOF'
 cask "kush" do
   version "$(VERSION)"
   sha256 "REPLACE_WITH_SHA256"

   url "https://github.com/grimdork/kush/releases/download/$(VERSION)/kush_$(VERSION)_darwin_amd64.tar.gz"
   name "kush"
   desc "kush — small interactive shell"
   homepage "https://github.com/grimdork/kush"

   binary "kush"

   zap trash: "~/.kush_history"
 end
 EOF
	@echo "Wrote $(DIST_DIR)/kush_cask.rb — update URL/sha256 as needed."

# Create a GitHub release draft and upload artifacts. Requires gh CLI and that you are authenticated.
# This is a convenience target; it will not sign artifacts.
release-draft: dist
	@echo "Creating GitHub release draft for $(VERSION)..."
	@gh release create $(VERSION) $(DIST_DIR)/* --title "$(VERSION)" --notes "Release $(VERSION)" --draft || echo "gh CLI failed or not configured"
	@echo "Release draft created (or failed)."
