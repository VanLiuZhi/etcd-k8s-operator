# ETCD Kubernetes Operator Makefile
# 基于core/etcd-operator重构的简化版本

# 镜像配置
IMG ?= etcd-operator:dev-v1.0

# Kubernetes版本配置 (支持1.28+)
ENVTEST_K8S_VERSION = 1.28.0

# Go工具路径配置
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# 容器工具配置 (Docker/Podman)
CONTAINER_TOOL ?= docker

# Shell配置
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ 帮助信息

.PHONY: help
help: ## 显示帮助信息
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ 开发工具

.PHONY: manifests
manifests: controller-gen ## 生成CRD、RBAC等Kubernetes配置文件
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./api/..." paths="./internal/..." paths="./cmd/..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## 生成deepcopy等代码
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./api/..." paths="./internal/..." paths="./cmd/..."

.PHONY: fmt
fmt: ## 格式化Go代码
	go fmt ./...

.PHONY: vet
vet: ## 运行go vet检查代码
	go vet ./...

##@ 测试

.PHONY: test
test: manifests generate fmt vet envtest ## 运行单元测试
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out

.PHONY: test-coverage
test-coverage: test ## 生成测试覆盖率报告
	go tool cover -html=cover.out -o coverage.html
	@echo "📊 测试覆盖率报告已生成: coverage.html"

##@ 本地开发

.PHONY: kind-create
kind-create: ## 创建Kind开发集群
	kind create cluster --name etcd-operator-dev

.PHONY: kind-delete
kind-delete: ## 删除Kind开发集群
	kind delete cluster --name etcd-operator-dev

.PHONY: kind-status
kind-status: ## 查看集群和CRD状态
	@echo "📊 集群状态:"
	@kubectl get nodes
	@echo ""
	@echo "🔧 EtcdCluster资源:"
	@kubectl get etcdclusters.k8s.etcd.lz -A || echo "暂无EtcdCluster资源"

##@ 重构后的便捷命令

.PHONY: dev-setup
dev-setup: ## 完整开发环境设置
	@echo "🚀 设置完整开发环境..."
	$(MAKE) kind-create
	@echo "⏳ 等待集群就绪..."
	@sleep 10
	kubectl wait --for=condition=Ready nodes --all --timeout=300s
	$(MAKE) install
	@echo "🎉 开发环境就绪!"
	@echo ""
	@echo "📋 快速命令:"
	@echo "   kubectl apply -f config/samples/"
	@echo "   make kind-status"

.PHONY: clean
clean: ## 清理生成的文件和资源
	@echo "🧹 清理生成的文件..."
	rm -f bin/manager
	rm -f cover.out coverage.html
	rm -rf dist/
	@echo "✅ 清理完成"

.PHONY: sample
sample: ## 创建示例EtcdCluster资源
	kubectl apply -f config/samples/etcd_v1alpha1_etcdcluster.yaml
	@echo "✅ 示例资源已创建"

.PHONY: sample-delete
sample-delete: ## 删除示例EtcdCluster资源
	kubectl delete -f config/samples/etcd_v1alpha1_etcdcluster.yaml --ignore-not-found=true
	@echo "✅ 示例资源已删除"
##@ 构建和运行

.PHONY: build
build: manifests generate fmt vet ## 构建管理器二进制文件
	go build -o bin/manager cmd/main.go

.PHONY: run
run: manifests generate fmt vet ## 在本地运行控制器
	go run ./cmd/main.go

.PHONY: docker-build
docker-build: ## 构建Docker镜像
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## 推送Docker镜像
	$(CONTAINER_TOOL) push ${IMG}

##@ 代码质量

.PHONY: lint
lint: golangci-lint ## 运行代码检查
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## 运行代码检查并自动修复
	$(GOLANGCI_LINT) run --fix

##@ 部署管理

.PHONY: install
install: manifests kustomize ## 安装CRD到k8s集群
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## 从k8s集群卸载CRD
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=true -f -

.PHONY: deploy
deploy: manifests kustomize ## 部署控制器到k8s集群
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: kustomize ## 从k8s集群卸载控制器
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=true -f -

##@ 依赖工具

# 本地工具安装目录
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

# 工具二进制文件路径
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)

# 工具版本配置
KUSTOMIZE_VERSION ?= v5.4.1
CONTROLLER_TOOLS_VERSION ?= v0.15.0
ENVTEST_VERSION ?= release-0.18
GOLANGCI_LINT_VERSION ?= v1.57.2

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## 下载kustomize工具
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## 下载controller-gen工具
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## 下载setup-envtest工具
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## 下载golangci-lint工具
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

# go工具安装函数
# $1 - 目标路径和二进制文件名
# $2 - 可安装的包URL
# $3 - 包的特定版本
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "正在下载 $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
