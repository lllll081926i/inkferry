# InkFerry builds are pure Go: every target cross-compiles without CGO.

GO      ?= go
DIST    ?= dist
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
UPX     ?= upx
ICON    ?= assets/app.ico
LDFLAGS = -trimpath -ldflags "-s -w -X main.version=$(VERSION)"
WINLDFLAGS = -trimpath -ldflags "-s -w -H windowsgui -X main.version=$(VERSION)"

.PHONY: all windows linux macos test vet fmt clean icons upx package

all: windows linux macos

windows:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(WINLDFLAGS) -o $(DIST)/InkFerry.exe .
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build $(WINLDFLAGS) -o $(DIST)/InkFerry-arm64.exe .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST)/ferry.exe ./cmd/ferry
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST)/ferryinstall.exe ./cmd/ferryinstall

linux:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST)/inkferry-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST)/ferry-linux-amd64 ./cmd/ferry
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(DIST)/ferryinstall-linux-amd64 ./cmd/ferryinstall

macos:
	@mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST)/InkFerry-macos-arm64 .
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST)/ferry-macos-arm64 ./cmd/ferry
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(DIST)/ferryinstall-macos-arm64 ./cmd/ferryinstall

# Build every supported target as one self-contained executable per platform
# and write SHA256SUMS.txt. No archives, no documentation, no extra tools.
package:
	VERSION="$(VERSION)" ./scripts/package.sh

test:
	CGO_ENABLED=0 $(GO) test ./...

vet:
	CGO_ENABLED=0 $(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Requires ImageMagick (convert) and rsrc. The checked-in .syso files already
# carry the icon for normal builds, so this is only needed after icon changes.
icons:
	@mkdir -p /tmp/inkferry-icon && $(GO) run ./cmd/mkicon /tmp/inkferry-icon
	convert /tmp/inkferry-icon/icon_256.png /tmp/inkferry-icon/icon_128.png /tmp/inkferry-icon/icon_64.png \
		/tmp/inkferry-icon/icon_48.png /tmp/inkferry-icon/icon_40.png /tmp/inkferry-icon/icon_32.png \
		/tmp/inkferry-icon/icon_24.png /tmp/inkferry-icon/icon_20.png /tmp/inkferry-icon/icon_16.png $(ICON)
	cp /tmp/inkferry-icon/icon_256.png assets/icon.png
	rsrc -arch amd64 -ico $(ICON) -o rsrc_windows_amd64.syso
	rsrc -arch arm64 -ico $(ICON) -o rsrc_windows_arm64.syso

upx: windows
	$(UPX) -9 -o $(DIST)/InkFerry-upx.exe $(DIST)/InkFerry.exe

clean:
	rm -rf $(DIST)
