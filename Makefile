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
