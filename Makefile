MODULE   = $(shell env GO111MODULE=on $(GO) list -m)
DATE    ?= $(shell date +%FT%T%z)
VERSION ?= $(shell git describe --tags --always --dirty 2> /dev/null || \
			cat $(CURDIR)/VERSION 2> /dev/null || echo v0)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
PKGS     = $(or $(PKG),$(shell env GO111MODULE=on $(GO) list ./...))
TESTPKGS = $(shell env GO111MODULE=on $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
LDFLAGS_VERSION = -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.GitBranch=$(BRANCH) -X main.BuildTime=$(DATE) 
BIN      = $(CURDIR)/.bin
GOLANGCI_LINT_CONFIG = $(CURDIR)/.golangci.yaml

PLATFORMS     = darwin linux windows
ARCHITECTURES = amd64 arm64

GO            = go
DOCKER        = docker
GOMOCK        = mockery
GOLANGCI_LINT = golangci-lint

TIMEOUT = 15
V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

export GO111MODULE=on
export CGO_ENABLED=0
export GOPROXY=https://proxy.golang.org

.PHONY: all
all: fmt lint test build

.PHONY: dependency
dependency: $(BIN); $(info $(M) downloading dependencies...) @ ## Build program binary
	$Q $(GO) mod download


.PHONY: build
build: dependency | $(BIN) ; $(info $(M) building executable...) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags "$(LDFLAGS_VERSION)" \
		-o $(BIN)/$(basename $(MODULE)) ./cmd/main.go

.PHONY: release
release: clean | $(BIN) ; $(info $(M) building executables for multiple os/arch...) @ ## Build program binary for paltforms and os
	$(foreach GOOS, $(PLATFORMS),\
		$(foreach GOARCH, $(ARCHITECTURES), \
			$(shell \
				if [ $(GOARCH) == "arm64" ] && [ $(GOOS) != "linux" ]; then exit 0; fi; \
				$(GO) build \
				-tags release \
				-ldflags "$(LDFLAGS_VERSION)" \
				-o $(BIN)/$(basename $(MODULE))_$(GOOS)_$(GOARCH) ./cmd/main.go || true)))



# Tools

$(BIN):
	@mkdir -p $@
$(BIN)/%: | $(BIN) ; $(info $(M) building $(PACKAGE)...)
	$Q tmp=$$(mktemp -d); \
	   env GO111MODULE=off GOPATH=$$tmp GOBIN=$(BIN) $(GO) get $(PACKAGE) \
		|| ret=$$?; \
	   rm -rf $$tmp ; exit $$ret

GOCOV = $(BIN)/gocov
$(BIN)/gocov: PACKAGE=github.com/axw/gocov/...

GOLINT = $(BIN)/golint
$(BIN)/golint: PACKAGE=golang.org/x/lint/golint

GOCOVXML = $(BIN)/gocov-xml
$(BIN)/gocov-xml: PACKAGE=github.com/AlekSi/gocov-xml

GO2XUNIT = $(BIN)/go2xunit
$(BIN)/go2xunit: PACKAGE=github.com/tebeka/go2xunit

GHR = $(BIN)/ghr
$(BIN)/ghr: PACKAGE=github.com/tcnksm/ghr

# build tools
build-tools: $(GOLINT) $(GOCOV) $(GOCOVXML) $(GO2XUNIT)

# Tests

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: fmt ; $(info $(M) running $(NAME:%=% )tests...) @ ## Run tests
	$Q env CGO_ENABLED=1 $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

test-xml: fmt lint | $(GO2XUNIT) ; $(info $(M) running xUnit tests...) @ ## Run tests with xUnit output
	$Q mkdir -p test
	$Q 2>&1 $(GO) test -timeout $(TIMEOUT)s -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

COVERAGE_MODE    = atomic
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML     = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML    = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage test-coverage-tools

test-coverage: COVERAGE_DIR := $(CURDIR)/.cover/coverage.$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
test-coverage: fmt; $(info $(M) running coverage tests...) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)
	$Q $(GO) test \
		-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $(TESTPKGS) | \
					grep '^$(MODULE)/' | grep -v mocks | \
					tr '\n' ',' | sed 's/,$$//') \
		-covermode=$(COVERAGE_MODE) \
		-coverprofile="$(COVERAGE_PROFILE)" $(TESTPKGS)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

.PHONY: golangci-lint
golangci-lint: | $(GOLANGCI_LINT) ; $(info $(M) running golangci-lint...) @ ## Run golangci-lint
	$Q $(GOLANGCI_LINT) run -v -c $(GOLANGCI_LINT_CONFIG) ./...

.PHONY: lint
lint: | $(GOLINT) ; $(info $(M) running golint...) @ ## Run golint
	$Q $(GOLINT) -set_exit_status $(PKGS)

.PHONY: fmt
fmt: ; $(info $(M) running gofmt...) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

# generate test mock for interfaces

.PHONY: mocks
mocks: $(info $(M) generating mocks...) ## Run mockery
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
github-release: release | $(GHR) ;$(info $(M) generating github release...) @ ## run ghr tool
ifndef GITHUB_TOKEN
	$(error GITHUB_TOKEN is undefined)
endif
	$Q $(GHR) \
		-t $(GITHUB_TOKEN) \
		-u alexei-led \
		-r pumba \
		-t "v$(RELEASE_TAG)" \
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