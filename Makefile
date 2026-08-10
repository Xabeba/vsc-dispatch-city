SHELL := /bin/sh
CLUSTER ?= delivery-lab
CONTEXT ?= k3d-$(CLUSTER)
TAG ?= local

.PHONY: test build images load deploy-03 port-forward

test:
	go test -race ./...
	cd apps/dashboard && npm run typecheck

build:
	go build ./...
	cd apps/dashboard && npm run build

images:
	TAG=$(TAG) ./scripts/build-images.sh

load:
	CLUSTER=$(CLUSTER) TAG=$(TAG) ./scripts/load-images.sh

deploy-03:
	kubectl --context $(CONTEXT) apply -k deploy/overlays/block-03-standalone

port-forward:
	kubectl --context $(CONTEXT) -n food-delivery port-forward service/dashboard 3000:3000 & \
	kubectl --context $(CONTEXT) -n food-delivery port-forward service/control-api 8081:8080
