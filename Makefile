# ------------------------------------------------------------------------------
# Configuration - Repository
# ------------------------------------------------------------------------------

REPO_URL ?= github.com/kong/kong-portal-controller
REPO_INFO ?= $(shell git config --get remote.origin.url)
TAG ?= $(shell git describe --tags)

ifndef COMMIT
  COMMIT := $(shell git rev-parse --short HEAD)
endif

# ------------------------------------------------------------------------------
# Configuration - Golang
# ------------------------------------------------------------------------------

export GO111MODULE=on

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# ------------------------------------------------------------------------------
# Configuration - Tooling
# ------------------------------------------------------------------------------

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.7.0)

KUSTOMIZE = $(shell pwd)/bin/kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v4@v4.3.0)

CLIENT_GEN = $(shell pwd)/bin/client-gen
client-gen: ## Download client-gen locally if necessary.
	$(call go-get-tool,$(CLIENT_GEN),k8s.io/code-generator/cmd/client-gen@v0.21.3)

# go-get-tool will 'go get' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go get $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

# ------------------------------------------------------------------------------
# Build
# ------------------------------------------------------------------------------

all: build

.PHONY: clean
clean:
	@rm -rf build/
	@rm -rf testbin/
	@rm -rf bin/*
	@rm -f coverage*.out

.PHONY: build
build: generate fmt vet lint
	go build -a -o bin/manager -ldflags "-s -w \
		-X kong-portal-controller/internal/metadata.Release=$(TAG) \
		-X kong-portal-controller/internal/metadata.Commit=$(COMMIT) \
		-X kong-portal-controller/internal/metadata.Repo=$(REPO_INFO)" internal/cmd/main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: lint
lint: verify.tidy
	golangci-lint run -v

.PHONY: verify.tidy
verify.tidy:
	./hack/verify-tidy.sh

.PHONY: verify.repo
verify.repo:
	./hack/verify-repo.sh

.PHONY: verify.diff
verify.diff:
	./hack/verify-diff.sh

.PHONY: verify.versions
verify.versions:
	./hack/verify-versions.sh $(TAG)

.PHONY: verify.manifests
verify.manifests: verify.repo manifests manifests.single verify.diff

.PHONY: verify.generators
verify.generators: verify.repo generate verify.diff

# ------------------------------------------------------------------------------
# Build - Manifests
# ------------------------------------------------------------------------------

CRD_OPTIONS ?= "+crd:allowDangerousTypes=true"

.PHONY: manifests
manifests: manifests.crds manifests.single

.PHONY: manifests.crds
manifests.crds: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=kong-developer webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: manifests.single
manifests.single: kustomize ## Compose single-file deployment manifests from building blocks
	./hack/deploy/build-single-manifests.sh

# ------------------------------------------------------------------------------
# Build - Generators
# ------------------------------------------------------------------------------

.PHONY: generate
generate: generate.controllers generate.clientsets

.PHONY: generate.controllers
generate.controllers: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."
	go generate ./...

# this will generate the custom typed clients needed for end-users implementing logic in Go to use our API types.
.PHONY: generate.clientsets
generate.clientsets: client-gen
	@$(CLIENT_GEN) --go-header-file ./hack/boilerplate.go.txt \
		--clientset-name clientset \
		--input-base kong-portal-controller/pkg/apis/  \
		--input configuration/v1 \
		--input-dirs github.com/kong/kong-portal-controller/pkg/apis/v1/ \
		--output-base client-gen-tmp/ \
		--output-package kong-portal-controller/pkg/
	@rm -rf pkg/clientset/
	@mv client-gen-tmp/kong-portal-controller/pkg/clientset pkg/
	@rm -rf client-gen-tmp/

# ------------------------------------------------------------------------------
# Build - Container Images
# ------------------------------------------------------------------------------

REGISTRY ?= kong
IMGNAME ?= kong-portal-controller
IMAGE ?= $(REGISTRY)/$(IMGNAME)

.PHONY: container
container:
	docker buildx build \
    -f Dockerfile \
    --target distroless \
    --build-arg TAG=${TAG} --build-arg COMMIT=${COMMIT} \
    --build-arg REPO_INFO=${REPO_INFO} \
    -t ${IMAGE}:${TAG} .

.PHONY: container
debug-container:
	docker buildx build \
    -f Dockerfile \
    --target debug \
    --build-arg TAG=${TAG}-debug --build-arg COMMIT=${COMMIT} \
    --build-arg REPO_INFO=${REPO_INFO} \
    -t ${IMAGE}:${TAG} .

# ------------------------------------------------------------------------------
# Testing
# ------------------------------------------------------------------------------

NCPU ?= $(shell getconf _NPROCESSORS_ONLN)
PKG_LIST = ./pkg/...,./internal/...

.PHONY: test
test: test.unit

.PHONY: test.all
test.all: test.unit

.PHONY: test.unit
test.unit:
	@go test -v -race \
		-covermode=atomic \
		-coverpkg=$(PKG_LIST) \
		-coverprofile=coverage.unit.out \
		./internal/... \
		./pkg/...

# ------------------------------------------------------------------------------
# Operations - Local Deployment
# ------------------------------------------------------------------------------

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./internal/cmd/main.go

install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMAGE}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -
