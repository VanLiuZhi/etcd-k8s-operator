# Image URL to use all building/pushing image targets
# Use timestamp-based tag to ensure unique images for development
#TIMESTAMP := $(shell date +%Y%m%d-%H%M)
IMG ?= etcd-operator:dev-v1.0
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.28.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run unit tests.
	@echo "Running unit tests..."
	@scripts/test/run-unit-tests.sh

.PHONY: test-unit
test-unit: test ## Alias for unit tests.

.PHONY: test-integration
test-integration: manifests generate fmt vet envtest ## Run integration tests.
	@echo "Running integration tests..."
	@scripts/test/run-integration-tests.sh

.PHONY: test-e2e
test-e2e: manifests generate fmt vet ## Run end-to-end tests.
	@echo "Running end-to-end tests..."
	@scripts/test/run-e2e-tests.sh

.PHONY: test-all
test-all: ## Run all tests (unit, integration, e2e).
	@echo "Running all tests..."
	@scripts/test/test-all.sh

.PHONY: test-fast
test-fast: ## Run tests in fast mode (skip non-critical tests).
	@echo "Running tests in fast mode..."
	@scripts/test/test-all.sh --fast

.PHONY: test-setup
test-setup: ## Setup test environment.
	@echo "Setting up test environment..."
	@scripts/test/setup-test-env.sh

.PHONY: test-cleanup
test-cleanup: ## Cleanup test environment.
	@echo "Cleaning up test environment..."
	@scripts/test/cleanup-test-env.sh

.PHONY: test-coverage
test-coverage: manifests generate fmt vet envtest ## Run tests with coverage report.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /e2e) -coverprofile cover.out -covermode=atomic
	go tool cover -html=cover.out -o coverage.html

##@ Kind Development

.PHONY: dev-setup
dev-setup: ## Complete development setup: create cluster and deploy operator.
	@echo "🚀 Setting up complete development environment..."
	$(MAKE) kind-create
	@echo "⏳ Waiting for cluster to be ready..."
	@sleep 10
	kubectl wait --for=condition=Ready nodes --all --timeout=300s
	$(MAKE) kind-deploy
	@echo "🎉 Development environment ready!"
	@echo ""
	@echo "📋 Quick commands:"
	@echo "   kubectl get pods -A"
	@echo "   kubectl apply -f config/samples/"
	@echo "   make kind-logs  # View operator logs"

.PHONY: kind-logs
kind-logs: ## Show operator logs from Kind cluster.
	kubectl logs -n etcd-k8s-operator-system deployment/etcd-k8s-operator-controller-manager -f

.PHONY: kind-status
kind-status: ## Show Kind cluster and operator status.
	@echo "📊 Kind Cluster Status:"
	@kubectl get nodes
	@echo ""
	@echo "📦 Operator Pods:"
	@kubectl get pods -n etcd-k8s-operator-system
	@echo ""
	@echo "🔧 Custom Resources:"
	@kubectl get etcd,etcdbackup,etcdrestore -A || echo "No custom resources found"

.PHONY: kind-create
kind-create: ## Create a Kind cluster for operator development.
	kind create cluster --name etcd-operator-dev --config hack/kind-config.yaml

.PHONY: kind-delete
kind-delete: ## Delete the Kind development cluster.
	kind delete cluster --name etcd-operator-dev

.PHONY: kind-clean-images
kind-clean-images: ## Clean up old development images.
	@echo "🧹 Cleaning up old development images..."
	docker images --format "table {{.Repository}}:{{.Tag}}\t{{.CreatedAt}}" | grep "etcd-operator:dev-" | head -n -3 | awk '{print $$1}' | xargs -r docker rmi || true
	@echo "✅ Old images cleaned up"

.PHONY: show-image
show-image: ## Show current image name and tag.
	@echo "Current image: ${IMG}"
	@echo "Available etcd-operator images:"
	@docker images etcd-operator --format "table {{.Repository}}:{{.Tag}}\t{{.Size}}\t{{.CreatedAt}}" || echo "No etcd-operator images found"

.PHONY: kind-load
kind-load: ## Load the operator image into Kind cluster.
	@echo "Loading image ${IMG} into Kind cluster..."
	kind load docker-image ${IMG} --name etcd-operator-dev
	@echo "Image loaded successfully"

.PHONY: kind-deploy
kind-deploy: ## Build, load and deploy operator to Kind cluster with fresh image.
	@echo "🚀 Building and deploying operator to Kind cluster..."
	@echo "Using image: ${IMG}"
	$(MAKE) docker-build
	$(MAKE) kind-load
	$(MAKE) install
	$(MAKE) deploy
	@echo "✅ Operator deployed successfully to Kind cluster"
	@echo "📋 Useful commands:"
	@echo "   kubectl get pods -n etcd-k8s-operator-system"
	@echo "   kubectl logs -n etcd-k8s-operator-system deployment/etcd-k8s-operator-controller-manager"

.PHONY: deploy-test
deploy-test: kind-deploy ## Alias for kind-deploy (backward compatibility).

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

##@ Build

.PHONY: build
build: manifests generate fmt vet ## Build manager binary.
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	$(CONTAINER_TOOL) push ${IMG}

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name etcd-k8s-operator-builder
	$(CONTAINER_TOOL) buildx use etcd-k8s-operator-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm etcd-k8s-operator-builder
	rm Dockerfile.cross

.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > dist/install.yaml

##@ Refactored Testing

.PHONY: test-unit
test-unit: ## Run unit tests only (new refactored tests)
	go test ./test/unit/... -v -coverprofile=unit.out

.PHONY: test-unit-coverage
test-unit-coverage: test-unit ## Generate unit test coverage report
	go tool cover -html=unit.out -o unit-coverage.html
	@echo "Unit test coverage report generated: unit-coverage.html"

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: deploy-files
deploy-files: manifests kustomize ## Generate deployment files without applying to cluster.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default > all-deploy.yaml
	@echo "Deployment files generated: all-deploy.yaml"

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.4.1
CONTROLLER_TOOLS_VERSION ?= v0.15.0
ENVTEST_VERSION ?= release-0.18
GOLANGCI_LINT_VERSION ?= v1.57.2

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
