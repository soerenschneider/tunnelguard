BUILD_DIR = builds
MODULE = github.com/soerenschneider/tunnelguard
BINARY_NAME = tunnelguard
CHECKSUM_FILE = checksum.sha256
SIGNATURE_KEYFILE = ~/.signify/github.sec
DOCKER_PREFIX = ghcr.io/soerenschneider

tests:
	go test ./... -covermode=atomic -coverprofile=coverage.out -race
	go tool cover -html=coverage.out -o=coverage.html
	go tool cover -func=coverage.out -o=coverage.out

clean:
	git diff --quiet || { echo 'Dirty work tree' ; false; }
	rm -rf ./$(BUILD_DIR)

build: version-info
	CGO_ENABLED=0 go build -ldflags="-X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}' -X 'main.GoVersion=${GO_VERSION}'" -o $(BINARY_NAME) .

release: clean version-info cross-build
	cd $(BUILD_DIR) && sha256sum * > $(CHECKSUM_FILE) && cd -

signed-release: release
	pass keys/signify/github | signify -S -s $(SIGNATURE_KEYFILE) -m $(BUILD_DIR)/$(CHECKSUM_FILE)
	gh-upload-assets -o soerenschneider -r vault-pki-cli -f ~/.gh-token builds

cross-build: version-info
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0       go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}' -X 'main.GoVersion=${GO_VERSION}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64     .
	GOOS=linux GOARCH=arm GOARM=6 CGO_ENABLED=0 go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}' -X 'main.GoVersion=${GO_VERSION}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-armv6     .
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0       go build -ldflags="-w -X 'main.BuildVersion=${VERSION}' -X 'main.CommitHash=${COMMIT_HASH}' -X 'main.GoVersion=${GO_VERSION}'" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-aarch64   .

version-info:
	$(eval VERSION := $(shell git describe --tags --abbrev=0 || echo "dev"))
	$(eval COMMIT_HASH := $(shell git rev-parse HEAD))
	$(eval GO_VERSION := $(shell go version | awk '{print $$3}' | sed 's/^go//'))

fmt:
	find . -iname "*.go" -exec go fmt {} \; 

pre-commit-init:
	pre-commit install
	pre-commit install --hook-type commit-msg

pre-commit-update:
	pre-commit autoupdate
