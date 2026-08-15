# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/heartwilltell/ghsecop:latest

.PHONY: all
all: build

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test: fmt vet
	go test ./... -count=1

.PHONY: build
build: fmt vet
	go build -o bin/manager ./cmd

.PHONY: run
run: build
	./bin/manager

.PHONY: docker-build
docker-build:
	docker build -t ${IMG} .

.PHONY: docker-push
docker-push:
	docker push ${IMG}

.PHONY: install
install:
	kubectl apply -f config/crd/bases/ghsecop.io_githubsecretsyncs.yaml

.PHONY: uninstall
uninstall:
	kubectl delete -f config/crd/bases/ghsecop.io_githubsecretsyncs.yaml --ignore-not-found=true

.PHONY: deploy
deploy: install
	kubectl apply -f config/manager/manager.yaml

.PHONY: undeploy
undeploy:
	kubectl delete -f config/manager/manager.yaml --ignore-not-found=true
	$(MAKE) uninstall

HELM_RELEASE ?= ghsecop
HELM_NAMESPACE ?= ghsecop-system

.PHONY: helm-lint
helm-lint:
	helm lint ./charts/ghsecop \
		--set credentials.connectToken=dummy \
		--set credentials.githubToken=dummy

.PHONY: helm-template
helm-template:
	helm template $(HELM_RELEASE) ./charts/ghsecop \
		--namespace $(HELM_NAMESPACE) \
		--set credentials.connectToken=dummy \
		--set credentials.githubToken=dummy

.PHONY: helm-install
helm-install:
	kubectl get ns $(HELM_NAMESPACE) >/dev/null 2>&1 || kubectl create namespace $(HELM_NAMESPACE)
	helm upgrade --install $(HELM_RELEASE) ./charts/ghsecop \
		--namespace $(HELM_NAMESPACE) \
		--set credentials.connectHost="$(OP_CONNECT_HOST)" \
		--set credentials.connectToken="$(OP_CONNECT_TOKEN)" \
		--set credentials.githubToken="$(GITHUB_TOKEN)"

.PHONY: helm-uninstall
helm-uninstall:
	helm uninstall $(HELM_RELEASE) --namespace $(HELM_NAMESPACE) --ignore-not-found
