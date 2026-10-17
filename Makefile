# Makefile untuk repo kubernetes-for-noob.
# Jalankan `make` tanpa argumen untuk melihat semua perintah.
# Variabel bisa diganti dari terminal, contoh: make rollout VERSION=v0.2.0

SHELL        := /bin/bash
.DEFAULT_GOAL := help

# --- Variabel yang boleh diubah ------------------------------------------------
APP          ?= tokoku-api
VERSION      ?= dev
GHCR_USER    ?= muhammadhasbiashshiddieqy
REGISTRY     ?= ghcr.io/$(GHCR_USER)
PLATFORMS    ?= linux/amd64,linux/arm64

CLUSTER      ?= lab
NAMESPACE    ?= tokoku
KIND_CONFIG  ?= k8s/kind/kind-3node.yaml
BASE_URL     ?= http://localhost:8081
N            ?= 3

# --- Turunan (jangan diubah) ---------------------------------------------------
LOCAL_IMG    := $(APP):$(VERSION)
REMOTE_IMG   := $(REGISTRY)/$(APP):$(VERSION)
LDFLAGS      := -s -w -X main.version=$(VERSION)
IMAGE_TAR    := .image.tar

.PHONY: help run build test fmt lint docker-build docker-run ghcr-login push \
	    kind-create kind-delete kind-load context up down up-cilium \
	    deploy undeploy status logs events describe port-forward smoke \
	    rollout rollback history scale restart \
	    cilium-install metrics-server nodeport hpa pdb watch-hpa loadtest

help: ## Tampilkan daftar perintah
	@awk 'BEGIN {FS = ":.*##"; printf "Pemakaian: make \033[36m<target>\033[0m [VAR=nilai]\n"} \
	    /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0, 5)} \
	    /^[a-zA-Z0-9_-]+:.*##/ {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Aplikasi Go (lokal)

run: ## Jalankan API di laptop (http://localhost:8080)
	PORT=8080 APP_ENV=local go run -ldflags "$(LDFLAGS)" ./cmd/api

build: ## Compile binary ke bin/api
	go build -ldflags "$(LDFLAGS)" -o bin/api ./cmd/api

test: ## Jalankan unit test
	go test ./... -race -count=1

fmt: ## Format semua kode Go
	gofmt -w .

lint: ## Cek format dan go vet
	@test -z "$$(gofmt -l .)" || (echo "File belum diformat, jalankan: make fmt" && gofmt -l . && exit 1)
	go vet ./...

##@ Image container

docker-build: ## Build image lokal (APP:VERSION), untuk arsitektur laptop
	docker build --build-arg VERSION=$(VERSION) -t $(LOCAL_IMG) .

docker-run: ## Jalankan image di Docker (http://localhost:8080)
	docker run --rm -p 8080:8080 --name $(APP) $(LOCAL_IMG)

ghcr-login: ## Petunjuk login ke GitHub Container Registry (tanpa kartu)
	@echo "1. Buat Personal Access Token (classic) di GitHub dengan scope write:packages"
	@echo "2. export GITHUB_TOKEN=<token kamu>   (jangan ditulis di file atau di-commit)"
	@echo "3. echo \$$GITHUB_TOKEN | docker login ghcr.io -u $(GHCR_USER) --password-stdin"

push: require-version ## Build multi-arch (amd64+arm64) dan push ke GHCR. Wajib: VERSION=v0.1.0
	docker buildx build --platform $(PLATFORMS) --build-arg VERSION=$(VERSION) -t $(REMOTE_IMG) --push .
	@echo "Terkirim: $(REMOTE_IMG)"

##@ Cluster kind

kind-create: ## Buat cluster kind 3 node (lewati jika sudah ada)
	@kind get clusters 2>/dev/null | grep -qx "$(CLUSTER)" && echo "Cluster '$(CLUSTER)' sudah ada" || kind create cluster --name $(CLUSTER) --config $(KIND_CONFIG)

kind-delete: ## Hapus cluster kind
	kind delete cluster --name $(CLUSTER)

kind-load: ## Muat image lokal ke semua node kind (via docker save, aman untuk containerd image store)
	docker save $(LOCAL_IMG) -o $(IMAGE_TAR)
	kind load image-archive $(IMAGE_TAR) --name $(CLUSTER)
	@rm -f $(IMAGE_TAR)

context: ## Tampilkan context kubectl aktif (cek sebelum apply/delete!)
	@kubectl config current-context

up: kind-create docker-build kind-load deploy ## Satu perintah: cluster + build + load + deploy

down: kind-delete ## Hapus seluruh cluster

##@ Deploy dan operasi harian

deploy: ## Terapkan manifest k8s/base ke cluster
	kubectl apply -k k8s/base
	kubectl -n $(NAMESPACE) rollout status deployment/api --timeout=120s

undeploy: ## Hapus semua resource Tokoku
	kubectl delete -k k8s/base --ignore-not-found

status: ## Lihat deployment, pod, service, dan hpa
	kubectl -n $(NAMESPACE) get deploy,rs,pods,svc,hpa -o wide

logs: ## Ikuti log semua pod api
	kubectl -n $(NAMESPACE) logs -l app=api --tail=30 -f --max-log-requests=10

events: ## Lihat event terbaru (tempat pertama mencari penyebab masalah)
	kubectl -n $(NAMESPACE) get events --sort-by=.lastTimestamp

describe: ## Describe deployment api
	kubectl -n $(NAMESPACE) describe deployment/api

port-forward: ## Akses service api di http://localhost:8081 (hanya menembus satu pod)
	kubectl -n $(NAMESPACE) port-forward svc/api 8081:80

smoke: ## Cek cepat endpoint (jalankan port-forward di terminal lain)
	@for p in / /healthz /readyz; do printf "%-10s " $$p; curl -s -o /dev/null -w "%{http_code}\n" $(BASE_URL)$$p; done
	@curl -s $(BASE_URL)/ ; echo

require-version:
	@test "$(VERSION)" != "dev" || (echo "Isi VERSION dengan tag, contoh: make $(MAKECMDGOALS) VERSION=v0.2.0" && exit 1)

rollout: require-version docker-build kind-load ## Rilis versi baru ke cluster. Wajib: VERSION=v0.2.0
	kubectl -n $(NAMESPACE) set image deployment/api api=$(LOCAL_IMG)
	kubectl -n $(NAMESPACE) rollout status deployment/api --timeout=120s

rollback: ## Kembali ke revisi sebelumnya
	kubectl -n $(NAMESPACE) rollout undo deployment/api
	kubectl -n $(NAMESPACE) rollout status deployment/api --timeout=120s

history: ## Riwayat revisi deployment
	kubectl -n $(NAMESPACE) rollout history deployment/api

scale: ## Ubah jumlah replika, contoh: make scale N=5
	kubectl -n $(NAMESPACE) scale deployment/api --replicas=$(N)

restart: ## Restart bertahap semua pod (rolling)
	kubectl -n $(NAMESPACE) rollout restart deployment/api

##@ Modul 7 (scaling dan production)

up-cilium: ## Cluster tanpa CNI bawaan + Cilium + build + load + deploy
	$(MAKE) kind-create KIND_CONFIG=k8s/kind/kind-3node-cilium.yaml
	$(MAKE) cilium-install
	$(MAKE) docker-build kind-load deploy

cilium-install: ## Pasang Cilium via Helm (untuk cluster kind-3node-cilium.yaml)
	helm repo add cilium https://helm.cilium.io/ >/dev/null 2>&1 || true
	helm repo update cilium
	helm upgrade --install cilium cilium/cilium --namespace kube-system --set image.pullPolicy=IfNotPresent --set ipam.mode=kubernetes
	kubectl wait --for=condition=Ready nodes --all --timeout=300s

metrics-server: ## Pasang metrics-server (dibutuhkan HPA dan kubectl top)
	helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/ >/dev/null 2>&1 || true
	helm repo update metrics-server
	helm upgrade --install metrics-server metrics-server/metrics-server --namespace kube-system --set 'args={--kubelet-insecure-tls}'

nodeport: ## Buka API di http://localhost:8090 (membagi traffic ke semua pod)
	kubectl apply -f k8s/extras/service-nodeport.yaml

hpa: ## Terapkan HorizontalPodAutoscaler
	kubectl apply -f k8s/extras/hpa.yaml

pdb: ## Terapkan PodDisruptionBudget
	kubectl apply -f k8s/extras/pdb.yaml

watch-hpa: ## Pantau HPA secara live
	kubectl -n $(NAMESPACE) get hpa api -w

loadtest: ## Uji beban dengan k6 lewat NodePort (butuh: make nodeport)
	k6 run -e BASE_URL=http://localhost:8090 loadtest/hpa.js
