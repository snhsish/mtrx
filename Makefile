BINARY=mtrx
PKG=./cmd/mtrx
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0)
LDFLAGS=-s -w -X main.version=$(VERSION)

.PHONY: build install test lint cross package clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install:
	go install -ldflags "$(LDFLAGS)" $(PKG)

test:
	go test ./...

lint:
	go vet ./...

cross:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-arm64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-amd64 $(PKG)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 $(PKG)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe $(PKG)
	$(MAKE) package

package:
	tar -czf dist/$(BINARY)-linux-amd64.tar.gz -C dist $(BINARY)-linux-amd64
	tar -czf dist/$(BINARY)-linux-arm64.tar.gz -C dist $(BINARY)-linux-arm64
	tar -czf dist/$(BINARY)-darwin-amd64.tar.gz -C dist $(BINARY)-darwin-amd64
	tar -czf dist/$(BINARY)-darwin-arm64.tar.gz -C dist $(BINARY)-darwin-arm64
	zip -j dist/$(BINARY)-windows-amd64.zip dist/$(BINARY)-windows-amd64.exe
	sha256sum dist/*.tar.gz dist/*.zip > dist/sha256sums.txt

clean:
	rm -rf $(BINARY) dist/
