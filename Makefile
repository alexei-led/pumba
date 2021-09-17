MODULE   = $(shell $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty 2> /dev/null || \
			cat $(CURDIR)/VERSION 2> /dev/null || echo v0)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
PKGS     = $(or $(PKG),$(shell $(GO) list ./...))
TESTPKGS = $(shell $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
LDFLAGS_VERSION = -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.GitBranch=$(BRANCH) -X main.BuildTime=$(DATE) 
BIN      = $(CURDIR)/.bin
GOLANGCI_LINT_CONFIG = $(CURDIR)/.golangci.yml

PLATFORMS     = darwin linux windows
ARCHITECTURES = amd64 arm64

TARGETOS   ?= linux
TARGETARCH ?= amd64


GO            = go
DOCKER        = docker
GOMOCK        = mockery
BATS          = bats
GOLANGCI_LINT = golangci-lint

TIMEOUT = 15
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

export CGO_ENABLED=0
export GOPROXY=https://proxy.golang.org

.PHONY: all
all: setup-tools fmt lint test build

.PHONY: dependency
dependency: ; $(info $(M) downloading dependencies...) @ ## Build program binary
	$Q $(GO) mod download


.PHONY: build
build: dependency | ; $(info $(M) building $(TARGETOS)/$(TARGETARCH) binary...) @ ## Build program binary
	$Q env GOOS=$(TARGETOS) GOARCH=$(TARGETARCH) $(GO) build \
		-tags release \
		-ldflags "$(LDFLAGS_VERSION)" \
		-o $(BIN)/$(basename $(MODULE)) ./cmd/main.go

.PHONY: release
release: clean ; $(info $(M) building binaries for multiple os/arch...) @ ## Build program binary for paltforms and os
	$(foreach GOOS, $(PLATFORMS),\
		$(foreach GOARCH, $(ARCHITECTURES), \
			$(shell \
				GOPROXY=$(GOPROXY) CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
				$(GO) build \
				-tags release \
				-ldflags "$(LDFLAGS_VERSION)" \
				-o $(BIN)/$(basename $(MODULE))_$(GOOS)_$(GOARCH) ./cmd/main.go || true)))

# Tools

setup-tools: setup-golint setup-golangci-lint setup-gocov setup-gocov-xml setup-go2xunit

setup-golint:
	$(GO) install golang.org/x/lint/golint@latest
setup-golangci-lint:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.42.1
setup-gocov:
	$(GO) install github.com/axw/gocov/gocov@latest
setup-gocov-xml:
	$(GO) install github.com/AlekSi/gocov-xml@latest
setup-go2xunit:
	$(GO) install github.com/tebeka/go2xunit@latest
setup-mockery:
	$(GO) install github.com/vektra/mockery/v2/
setup-ghr:
	$(GO) install github.com/tcnksm/ghr@latest

GOLINT=golint
GOCOV=gocov
GOCOVXML=gocov-xml
GO2XUNIT=go2xunit
GOMOCK=mockery
GHR=ghr

# Tests

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:                       ## Run tests with race detector
ifeq ($(GOOS)$(GOARCH),linuxamd64)
  ARGS=-race 
endif
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: ; $(info $(M) running $(NAME:%=% )tests...) @ ## Run tests
	$Q env CGO_ENABLED=1 $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: setup-go2xunit; $(info $(M) running xUnit tests...) @ ## Run tests with xUnit output
	$Q mkdir -p test
	$Q 2>&1 $(GO) test -timeout $(TIMEOUT)s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE    = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML     = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML    = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage
test-coverage: COVERAGE_DIR := $(CURDIR)/.cover/coverage.$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
test-coverage: setup-gocov setup-gocov-xml; $(info $(M) running coverage tests...) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GO) test \
		-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $(TESTPKGS) | \
					grep '^$(MODULE)/' | grep -v mocks | \
					tr '\n' ',' | sed 's/,$$//') \
		-covermode=$(COVERAGE_MODE) \
		-coverprofile="$(COVERAGE_PROFILE)" $(TESTPKGS)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

# urun integration tests
.PHONY: integration-tests
integration-tests: build ; $(info $(M) running integration tests with bats...) @ ## Run bats tests
	$Q PATH=$(BIN)/$(dir $(MODULE)):$(PATH) pumba --version
	$Q PATH=$(BIN)/$(dir $(MODULE)):$(PATH) $(BATS) tests

.PHONY: golangci-lint
golangci-lint: setup-golangci-lint; $(info $(M) running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI_LINT) run -v -c $(GOLANGCI_LINT_CONFIG) ./...

.PHONY: lint
lint: setup-golint; $(info $(M) running golint...) @ ## Run golint
	$Q $(GOLINT) -set_exit_status $(PKGS)

.PHONY: fmt
fmt: ; $(info $(M) running gofmt...) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

# generate test mock for interfaces
.PHONY: mocks
mocks: setup-mockery; $(info $(M) generating mocks...) @ ## Run mockery
	$Q $(GOMOCK) --dir pkg/chaos/docker --all
	$Q $(GOMOCK) --dir pkg/container --inpackage --all
	$Q $(GOMOCK) --dir $(call source_of,github.com/docker/engine)/client --name ContainerAPIClient
	$Q $(GOMOCK) --dir $(call source_of,github.com/docker/engine)/client --name ImageAPIClient
	$Q $(GOMOCK) --dir $(call source_of,github.com/docker/engine)/client --name APIClient

# generate CHANGELOG.md changelog file
.PHONY: changelog
changelog: $(DOCKER) ; $(info $(M) generating changelog...)	@ ## Generating CAHNGELOG.md
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	$Q $(DOCKER) run -it --rm -v $(CURDIR):/usr/local/src/pumba -w /usr/local/src/pumba ferrarimarco/github-changelog-generator --user alexei-led --project pumba --token $(GITHUB_TOKEN)

# generate github release
.PHONY: github-release
github-release: setup-ghr | release ;$(info $(M) generating github release...) @ ## run ghr tool
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	$Q $(GHR) \
		-t $(GITHUB_TOKEN) \
		-u alexei-led \
		-r pumba \
		-n "v$(RELEASE_TAG)" \
		-b "$(TAG_MESSAGE)" \
		-prerelease \
		-draft \
		$(RELEASE_TAG) \
		$(BIN)/$(dir $(MODULE))

# Misc

.PHONY: clean
clean: ; $(info $(M) cleaning...)	@ ## Cleanup everything
	@rm -rf $(BIN)
	@rm -rf .cover/coverage.*

.PHONY: help
help:
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: debug
debug:
	@echo $(LDFLAGS_VERSION)

# helper function: find module path
define source_of
	$(shell go mod download -json | jq -r 'select(.Path == "$(1)").Dir' | tr '\\' '/'  2> /dev/null)
endef
