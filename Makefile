SHELL    := /bin/bash
GO       := go
DOCKER   := docker
MOCK     := mockery
BATS     := bats
LINT     := $(shell go env GOPATH)/bin/golangci-lint
GOCOV    := gocov
GOCOVXML := gocov-xml
GOUNIT   := go-junit-report
GOMOCK   := $(shell $(GO) env GOPATH)/bin/mockery
TIMEOUT  := 15

MODULE   := $(shell $(GO) list -m)
DATE     ?= $(shell date "+%Y-%m-%d %H:%M %Z")
VERSION  ?= $(shell git describe --tags --always --dirty 2> /dev/null || cat $(CURDIR)/VERSION 2> /dev/null || echo v0)
COMMIT   ?= $(or $(shell git rev-parse --short HEAD 2>/dev/null), $(or $(subst 1,7,$(GITHUB_SHA)), unknown))
BRANCH   ?= $(or $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null), $(or $(GITHUB_REF_NAME), master))
PKGS     := $(or $(shell $(GO) list ./...), $(PKG))
TESTPKGS := $(shell $(GO) list -f \
			'{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' \
			$(PKGS))
LINT_CONFIG := $(CURDIR)/.golangci.yaml
LDFLAGS_VERSION := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.branch=$(BRANCH) -X \"main.buildTime=$(DATE)\"
BIN        := $(CURDIR)/.bin
PUMBA_BIN  := $(BIN)/$(basename $(MODULE))
ADVANCED_TEST_BIN = $(BIN)/integration-tests-$(LOCAL_TARGETARCH).test
TARGETOS   := $(or $(TARGETOS), linux)
TARGETARCH := $(or $(TARGETARCH), amd64)
HOSTOS     := $(shell uname -s)
LOCAL_TARGETARCH ?= $(shell uname -m | sed -e 's/aarch64/arm64/' -e 's/x86_64/amd64/')
LOCAL_PODMAN_MACHINE ?= pumba-podman

# platforms and architectures for release
# Pumba is a Linux-container chaos tool. The CLI talks to Linux container
# runtime sockets (Docker/containerd/Podman) and injects sidecars into
# Linux netns/cgroups. Windows is intentionally not built and not supported.
# darwin binaries are provided for developer ergonomics (talking to Linux
# containers in a remote VM).
PLATFORMS     = darwin linux
ARCHITECTURES = amd64 arm64

V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

export CGO_ENABLED=0
export GOPROXY=https://proxy.golang.org
export GOOS=$(TARGETOS)
export GOARCH=$(TARGETARCH)

.PHONY: all
all: setup-tools fmt lint test build

.PHONY: dependency
dependency: ; $(info $(M) downloading dependencies...) @ ## Build program binary
	$Q $(GO) mod download

.PHONY: build
build: dependency | ; $(info $(M) building $(GOOS)/$(GOARCH) binary...) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags "$(LDFLAGS_VERSION)" \
		-o $(BIN)/$(basename $(MODULE)) ./cmd

.PHONY: release
# Recipe-side bash loop with `set -euo pipefail`: any per-platform `go build`
# failure aborts the recipe loudly. The previous `$(foreach $(shell ...))`
# pattern evaluated at make-parse time and silently swallowed exit codes,
# leaving missing binaries undetectable until release-time.
release: clean ; $(info $(M) building binaries for multiple os/arch...) @ ## Build program binary for platforms and architectures
	@set -euo pipefail; \
	for goos in $(PLATFORMS); do \
	  for goarch in $(ARCHITECTURES); do \
	    out="$(BIN)/pumba_$${goos}_$${goarch}"; \
	    echo "  building $${goos}/$${goarch} -> $${out}"; \
	    GOPROXY=$(GOPROXY) CGO_ENABLED=$(CGO_ENABLED) GOOS=$$goos GOARCH=$$goarch \
	      $(GO) build -tags release -ldflags "$(LDFLAGS_VERSION)" \
	      -o "$$out" ./cmd || { echo "::error::build failed: $${goos}/$${goarch}"; exit 1; }; \
	  done; \
	done

# Tools

setup-tools: setup-lint setup-gocov setup-gocov-xml setup-go-junit-report

setup-lint:
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.0
setup-gocov:
	$(GO) install github.com/axw/gocov/gocov@v1.1.0
setup-gocov-xml:
	$(GO) install github.com/AlekSi/gocov-xml@latest
setup-go-junit-report:
	$(GO) install github.com/jstemmer/go-junit-report/v2@latest
setup-mockery:
	$(GO) install github.com/vektra/mockery/v2@v2.53.5

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
	$Q env CGO_ENABLED=1 GOOS= GOARCH= $(GO) test -timeout $(TIMEOUT)s $(ARGS) $(TESTPKGS)

COVERAGE_MODE    = atomic
COVERAGE_DIR 	 = $(CURDIR)/.cover
COVERAGE_PROFILE = $(COVERAGE_DIR)/profile.out
COVERAGE_XML     = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML    = $(COVERAGE_DIR)/index.html

.PHONY: test-coverage
test-coverage: setup-go-junit-report setup-gocov setup-gocov-xml; $(info $(M) running coverage tests...) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)
	$Q set -o pipefail; env CGO_ENABLED=1 GOOS= GOARCH= $(GO) test -v -cover \
		-coverpkg=$$(env CGO_ENABLED=1 GOOS= GOARCH= $(GO) list -f '{{ join .Deps "\n" }}' $(TESTPKGS) | \
					grep '^$(MODULE)/' | grep -v mocks | \
					tr '\n' ',' | sed 's/,$$//') \
		-covermode=$(COVERAGE_MODE) \
		-coverprofile="$(COVERAGE_PROFILE)" $(TESTPKGS) 2>&1 | tee $(COVERAGE_DIR)/tests.output
	$(GOUNIT) -set-exit-code -in $(COVERAGE_DIR)/tests.output -out $(COVERAGE_DIR)/tests.xml
	$Q $(GO) tool cover -func="$(COVERAGE_PROFILE)"
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)

ifeq ($(HOSTOS),Darwin)
.PHONY: integration-tests integration-tests-all integration-tests-local-docker integration-tests-local-docker-all integration-tests-local-containerd integration-tests-local-podman build-local-linux install-pumba-colima install-pumba-podman ensure-colima-test-deps ensure-podman-test-deps
integration-tests: integration-tests-local-docker integration-tests-local-containerd integration-tests-local-podman ## Run local macOS integration tests in runtime VMs
integration-tests-all: integration-tests-local-docker-all integration-tests-local-containerd integration-tests-local-podman ## Run all local macOS integration tests in runtime VMs

build-local-linux: ; $(info $(M) building linux/$(LOCAL_TARGETARCH) binary for local VMs...) @
	$Q $(MAKE) build TARGETOS=linux TARGETARCH=$(LOCAL_TARGETARCH)

install-pumba-colima: build-local-linux ; $(info $(M) installing pumba in Colima VM...) @
	$Q colima ssh -- sudo install -m 755 $(PUMBA_BIN) /usr/local/bin/pumba
	$Q colima ssh -- pumba --version

install-pumba-podman: build-local-linux ; $(info $(M) installing pumba in Podman VM...) @
	$Q podman machine ssh $(LOCAL_PODMAN_MACHINE) sudo install -m 755 $(PUMBA_BIN) /usr/local/bin/pumba
	$Q podman machine ssh $(LOCAL_PODMAN_MACHINE) pumba --version

ensure-colima-test-deps: ; $(info $(M) checking Colima integration test dependencies...) @
	$Q colima status >/dev/null
	$Q colima ssh -- bash -lc 'set -e; if ! command -v bats >/dev/null 2>&1; then sudo apt-get update -qq && sudo apt-get install -y -qq bats git; fi; sudo mkdir -p /usr/local/lib; if [ ! -d /usr/local/lib/bats-support ]; then sudo git clone --depth 1 https://github.com/bats-core/bats-support.git /usr/local/lib/bats-support; fi; if [ ! -d /usr/local/lib/bats-assert ]; then sudo git clone --depth 1 https://github.com/bats-core/bats-assert.git /usr/local/lib/bats-assert; fi; for img in docker.io/library/alpine:latest docker.io/nicolaka/netshoot:latest ghcr.io/alexei-led/pumba-alpine-nettools:latest ghcr.io/alexei-led/stress-ng:latest; do docker image inspect "$$img" >/dev/null 2>&1 || docker pull "$$img"; sudo ctr -n moby i ls -q | grep -qx "$$img" || sudo ctr -n moby i pull "$$img"; done; sudo ctr i ls -q | grep -qx docker.io/library/alpine:latest || sudo ctr i pull docker.io/library/alpine:latest'

ensure-podman-test-deps: ; $(info $(M) checking Podman integration test dependencies...) @
	$Q podman machine ssh $(LOCAL_PODMAN_MACHINE) 'set -e; sudo mkdir -p /usr/local/lib /usr/local/bin; if ! command -v bats >/dev/null 2>&1; then if [ ! -d /usr/local/lib/bats-core ]; then sudo git clone --depth 1 --branch v1.13.0 https://github.com/bats-core/bats-core.git /usr/local/lib/bats-core; fi; sudo ln -sf /usr/local/lib/bats-core/bin/bats /usr/local/bin/bats; fi; if [ ! -d /usr/local/lib/bats-support ]; then sudo git clone --depth 1 https://github.com/bats-core/bats-support.git /usr/local/lib/bats-support; fi; if [ ! -d /usr/local/lib/bats-assert ]; then sudo git clone --depth 1 https://github.com/bats-core/bats-assert.git /usr/local/lib/bats-assert; fi; for img in docker.io/library/alpine:latest docker.io/nicolaka/netshoot:latest ghcr.io/alexei-led/pumba-alpine-nettools:latest ghcr.io/alexei-led/stress-ng:latest; do sudo podman image exists "$$img" || sudo podman pull "$$img"; done'

integration-tests-local-docker: install-pumba-colima ensure-colima-test-deps ; $(info $(M) running Docker integration tests in Colima VM...) @
	$Q colima ssh -- bash -O extglob -lc 'cd $(CURDIR) && sudo env "PATH=/usr/local/bin:$$PATH" bats --tap --timing --print-output-on-failure tests/!(containerd_*|podman_*|stress).bats'

integration-tests-local-docker-all: install-pumba-colima ensure-colima-test-deps ; $(info $(M) running all Docker integration tests in Colima VM...) @
	$Q colima ssh -- bash -O extglob -lc 'cd $(CURDIR) && sudo env "PATH=/usr/local/bin:$$PATH" bats --tap --timing --print-output-on-failure tests/!(containerd_*|podman_*).bats'

integration-tests-local-containerd: install-pumba-colima ensure-colima-test-deps ; $(info $(M) running containerd integration tests in Colima VM...) @
	$Q colima ssh -- bash -lc 'cd $(CURDIR) && sudo env "PATH=/usr/local/bin:$$PATH" bats --tap --timing --print-output-on-failure tests/containerd_*.bats'

integration-tests-local-podman: install-pumba-podman ensure-podman-test-deps ; $(info $(M) running Podman integration tests in Podman VM...) @
	$Q podman machine ssh $(LOCAL_PODMAN_MACHINE) 'cd $(CURDIR) && sudo env "PATH=/usr/local/bin:$$PATH" bats --tap --timing --print-output-on-failure tests/podman_*.bats'
else
# run integration tests
.PHONY: integration-tests
integration-tests: build ; $(info $(M) running integration tests with bats...) @ ## Run bats tests
	$Q PATH="$(BIN)/$(dir $(MODULE)):$(PATH)" pumba --version
	$Q PATH="$(BIN)/$(dir $(MODULE)):$(PATH)" $(SHELL) tests/run_tests.sh
	
# run all integration tests including stress tests
.PHONY: integration-tests-all
integration-tests-all: build ; $(info $(M) running all integration tests with bats...) @ ## Run all bats tests including stress tests
	$Q PATH="$(BIN)/$(dir $(MODULE)):$(PATH)" pumba --version
	$Q PATH="$(BIN)/$(dir $(MODULE)):$(PATH)" $(SHELL) tests/run_tests.sh --all
endif

ifeq ($(HOSTOS),Darwin)
# run advanced Go integration tests (requires Docker, sudo for nsenter/containerd)
.PHONY: integration-tests-advanced build-advanced-integration-linux
build-advanced-integration-linux: ; $(info $(M) building linux/$(LOCAL_TARGETARCH) advanced integration test binary...) @
	$Q env CGO_ENABLED=0 GOOS=linux GOARCH=$(LOCAL_TARGETARCH) $(GO) test -c -tags integration -o $(ADVANCED_TEST_BIN) ./tests/integration

integration-tests-advanced: install-pumba-colima ensure-colima-test-deps build-advanced-integration-linux ; $(info $(M) running advanced Go integration tests in Colima VM...) @ ## Run Go integration tests
	$Q colima ssh -- bash -lc 'cd $(CURDIR) && sudo env "PATH=/usr/local/bin:$$PATH" $(ADVANCED_TEST_BIN) -test.v -test.timeout=300s -test.parallel=1'
else
# run advanced Go integration tests (requires Docker, sudo for nsenter/containerd)
.PHONY: integration-tests-advanced
integration-tests-advanced: build ; $(info $(M) running advanced Go integration tests...) @ ## Run Go integration tests
	$Q CGO_ENABLED=0 $(GO) test -v -tags integration -timeout 300s -count=1 ./tests/integration/...
endif

.PHONY: lint
lint: setup-lint; $(info $(M) running golangci-lint...) @ ## Run golangci-lint
	$Q $(LINT) run -v -c $(LINT_CONFIG) ./...

.PHONY: fmt
fmt: ; $(info $(M) running gofmt...) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

# generate test mocks for interfaces (reads .mockery.yaml)
.PHONY: mocks
mocks: setup-mockery; $(info $(M) generating mocks...) @ ## Run mockery
	$Q $(GOMOCK)

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
	@echo $(BIN)/$(basename $(MODULE))
	@echo $(TARGETOS)/$(TARGETARCH)

# NetTools Docker images
NETTOOLS_REPO := ghcr.io/alexei-led/pumba
NETTOOLS_PLATFORMS := linux/amd64,linux/arm64

.PHONY: build-nettools-images
build-nettools-images: ; $(info $(M) building multi-arch nettools images...) @ ## Build multi-arch nettools images
	$Q $(DOCKER) buildx create --use --name nettools-builder --driver docker-container --bootstrap || true
	$Q $(DOCKER) buildx build --platform $(NETTOOLS_PLATFORMS) \
		-t $(NETTOOLS_REPO)/pumba-alpine-nettools:latest \
		-f $(CURDIR)/docker/alpine-nettools.Dockerfile \
		$(CURDIR)
	$Q $(DOCKER) buildx build --platform $(NETTOOLS_PLATFORMS) \
		-t $(NETTOOLS_REPO)/pumba-debian-nettools:latest \
		-f $(CURDIR)/docker/debian-nettools.Dockerfile \
		$(CURDIR)
	$Q $(DOCKER) buildx rm nettools-builder

.PHONY: build-local-nettools
build-local-nettools: ; $(info $(M) building local nettools images for local architecture...) @ ## Build local nettools images
	$Q $(DOCKER) build \
		-t pumba-alpine-nettools:local \
		-f $(CURDIR)/docker/alpine-nettools.Dockerfile \
		$(CURDIR)
	$Q $(DOCKER) build \
		-t pumba-debian-nettools:local \
		-f $(CURDIR)/docker/debian-nettools.Dockerfile \
		$(CURDIR)

.PHONY: push-nettools-images
push-nettools-images: ; $(info $(M) building and pushing multi-arch nettools images...) @ ## Build and push multi-arch nettools images
	@echo "Using repository: $(NETTOOLS_REPO)"
	@echo "Checking if already logged in to ghcr.io..."
	@$(DOCKER) buildx ls >/dev/null
	$Q $(DOCKER) buildx create --use --name nettools-builder --driver docker-container --bootstrap || true
	$Q $(DOCKER) buildx build --platform $(NETTOOLS_PLATFORMS) \
		-t $(NETTOOLS_REPO)/pumba-alpine-nettools:latest \
		--push \
		-f $(CURDIR)/docker/alpine-nettools.Dockerfile \
		$(CURDIR)
	$Q $(DOCKER) buildx build --platform $(NETTOOLS_PLATFORMS) \
		-t $(NETTOOLS_REPO)/pumba-debian-nettools:latest \
		--push \
		-f $(CURDIR)/docker/debian-nettools.Dockerfile \
		$(CURDIR)
	$Q $(DOCKER) buildx rm nettools-builder
