# Copyright 2021 Linka Cloud  All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

MODULE = go.linka.cloud/d2vm

REPOSITORY = linkacloud

TAG = $(shell git describe --tags --exact-match 2> /dev/null)
VERSION_SUFFIX = $(shell git diff --quiet || echo "-dev")
VERSION = $(shell git describe --tags --exact-match 2> /dev/null || echo "`git describe --tags $$(git rev-list --tags --max-count=1) 2> /dev/null || echo v0.0.0`-`git rev-parse --short HEAD`")$(VERSION_SUFFIX)
show-version:
	@echo $(VERSION)

GORELEASER_VERSION := v1.10.1
GORELEASER_URL := https://github.com/goreleaser/goreleaser/releases/download/$(GORELEASER_VERSION)/goreleaser_Linux_x86_64.tar.gz

BIN := $(PWD)/bin
export PATH := $(BIN):$(PATH)

CLI_REFERENCE_PATH := docs/reference

bin:
	@mkdir -p $(BIN)
	@curl -sL $(GORELEASER_URL) | tar -C $(BIN) -xz goreleaser

clean-bin:
	@rm -rf $(BIN)

DOCKER_IMAGE := linkacloud/d2vm

docker: docker-build docker-push

docker-push:
	@docker image push $(DOCKER_IMAGE):$(VERSION)
ifneq ($(TAG),)
	@docker image push $(DOCKER_IMAGE):latest
endif

docker-build:
	@docker image build -t $(DOCKER_IMAGE):$(VERSION) .
ifneq ($(TAG),)
	@docker image tag $(DOCKER_IMAGE):$(TAG) $(DOCKER_IMAGE):latest
endif

docker-run:
	@docker run --rm -i -t \
		--privileged \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD):/build \
		-w /build \
		$(DOCKER_IMAGE) bash

.PHONY: tests
tests:
	@go generate ./...
	@go list ./...| xargs go test -exec sudo -count=1 -timeout 20m -v

docs-up-to-date:
	@$(MAKE) docs
	@git diff --quiet -- docs ':(exclude)docs/reference/d2vm_run_qemu.md' || (git --no-pager diff -- docs ':(exclude)docs/reference/d2vm_run_qemu.md'; echo "Please regenerate the documentation with 'make docs'"; exit 1)

check-fmt:
	@[ "$(gofmt -l $(find . -name '*.go') 2>&1)" = "" ]

vet:
	@go list ./...|grep -v scratch|GOOS=linux xargs go vet

build-dev: docker-build .build

.build:
	@go generate ./...
	@go build -o d2vm -ldflags "-s -w -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildDate=$(shell date)'" ./cmd/d2vm

.PHONY: build-snapshot
build-snapshot: bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser build --snapshot --rm-dist --parallelism 8

.PHONY: release-snapshot
release-snapshot: bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser release --snapshot --rm-dist --skip-announce --skip-publish --parallelism 8

.PHONY: build
build: $(BIN) bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser build --rm-dist --parallelism 8

.PHONY: release
release: $(BIN) bin
	@VERSION=$(VERSION) IMAGE=$(DOCKER_IMAGE) goreleaser release --rm-dist --parallelism 8

.PHONY: examples
examples: build-dev
	@mkdir -p examples/build
	@for f in $$(find examples -type f -name '*Dockerfile' -maxdepth 1); do \
  		echo "Building $$f"; \
  		./d2vm build -o examples/build/$$(basename $$f|cut -d'.' -f1).qcow2 -f $$f examples; \
	  done
	@echo "Building examples/full/Dockerfile"
	@./d2vm build -o examples/build/full.qcow2 --build-arg=USER=adphi --build-arg=PASSWORD=adphi examples/full

docs: .build
	@rm -rf $(CLI_REFERENCE_PATH)
	@./d2vm docs $(CLI_REFERENCE_PATH)
