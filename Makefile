GO       := go
DOCKER   := docker
MOCK     := mockery
LINT     := golangci-lint
GOCOV    := gocov
GOCOVXML := gocov-xml
GOUNIT   := go-junit-report
GOMOCK   := mockery

MODULE   := $(shell $(GO) list -m)
DATE     ?= $(shell date "+%Y-%m-%d %H:%M %Z")
VERSION  ?= $(shell git describe --tags --always --dirty 2> /dev/null || cat $(CURDIR)/VERSION 2> /dev/null || echo v0)
COMMIT   ?= $(or $(shell git rev-parse --short HEAD 2>/dev/null), $(or $(subst 1,7,$(GITHUB_SHA)), unknown))
BRANCH   ?= $(or $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null), $(or $(GITHUB_REF_NAME), master))
PKGS     := $(or $(shell $(GO) list ./...), $(PKG))
LINT_CONFIG := $(CURDIR)/.golangci.yaml
LDFLAGS_VERSION := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.branch=$(BRANCH) -X \"main.buildTime=$(DATE)\"
BIN        := $(CURDIR)/bin
TARGETOS   := $(or $(TARGETOS), linux)
TARGETARCH := $(or $(TARGETARCH), amd64)

# platforms and architectures for release
PLATFORMS     = darwin linux windows
ARCHITECTURES = amd64 arm64

V = 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

export CGO_ENABLED=0
export GOPROXY=https://proxy.golang.org
export GOOS=$(TARGETOS)
export GOARCH=$(TARGETARCH)

.PHONY: all
all: setup-tools fmt lint build

.PHONY: dependency
dependency: ; $(info $(M) downloading dependencies...) @ ## Build program binary
	$Q $(GO) mod download

.PHONY: build
build: dependency ; $(info $(M) building $(GOOS)/$(GOARCH) binary...) @ ## Build program binary
	$Q $(GO) build \
		-tags release \
		-ldflags "$(LDFLAGS_VERSION)" \
		-o $(BIN)/$(basename $(MODULE)) ./cmd/main.go

.PHONY: release
release: clean ; $(info $(M) building binaries for multiple os/arch...) @ ## Build program binary for platforms and os
	$(foreach GOOS, $(PLATFORMS),\
		$(foreach GOARCH, $(ARCHITECTURES), \
			$(shell \
				GOPROXY=$(GOPROXY) CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
				$(GO) build \
				-tags release \
				-ldflags "$(LDFLAGS_VERSION)" \
				-o $(BIN)/$(basename $(MODULE))_$(GOOS)_$(GOARCH) ./cmd/main.go || true)))

# Tools

setup-tools: setup-lint setup-gocov setup-gocov-xml setup-go-junit-report

setup-lint:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.54.2
setup-gocov:
	$(GO) install github.com/axw/gocov/gocov@v1.1.0
setup-gocov-xml:
	$(GO) install github.com/AlekSi/gocov-xml@latest
setup-go-junit-report:
	$(GO) install github.com/jstemmer/go-junit-report/v2@latest
setup-mockery:
	$(GO) get github.com/vektra/mockery/v2@latest

.PHONY: lint
lint: setup-lint; $(info $(M) running golangci-lint...) @ ## Run golangci-lint
	$Q $(LINT) run -v -c $(LINT_CONFIG) ./...

.PHONY: fmt
fmt: ; $(info $(M) running gofmt...) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

# generate test mock for interfaces
.PHONY: mocks
mocks: setup-mockery; $(info $(M) generating mocks...) @ ## Run mockery
	$Q $(GOMOCK) --dir pkg/chaos/docker --all
	$Q $(GOMOCK) --dir pkg/container --inpackage --all
	$Q $(GOMOCK) --dir $(call source_of,github.com/docker/docker)/client --name ContainerAPIClient
	$Q $(GOMOCK) --dir $(call source_of,github.com/docker/docker)/client --name ImageAPIClient
	$Q $(GOMOCK) --dir $(call source_of,github.com/docker/docker)/client --name APIClient

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

# helper function: find module path
define source_of
	$(shell go mod download -json | jq -r 'select(.Path == "$(1)").Dir' | tr '\\' '/'  2> /dev/null)
endef
