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

VERSION_SUFFIX = $(shell git diff --quiet || echo "-dev")
VERSION = $(shell git describe --tags --exact-match 2> /dev/null || echo "`git describe --tags $$(git rev-list --tags --max-count=1) 2> /dev/null || echo v0.0.0`-`git rev-parse --short HEAD`")$(VERSION_SUFFIX)
show-version:
	@echo $(VERSION)

DOCKER_IMAGE := linkacloud/d2vm

docker: docker-build docker-push

docker-push:
	@docker image push -a $(DOCKER_IMAGE)

docker-build:
	@docker image build -t $(DOCKER_IMAGE):$(VERSION) .
	@echo $(VERSION)|grep -q '-' || docker image tag $(DOCKER_IMAGE):latest $(DOCKER_IMAGE):$(VERSION)

docker-run:
	@docker run --rm -i -t \
		--privileged \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD):/build \
		-w /build \
		$(DOCKER_IMAGE) bash

build:
	@go build -o d2vm -ldflags "-s -w -X '$(MODULE).Image=$(DOCKER_IMAGE)' -X '$(MODULE).Version=$(VERSION)' -X '$(MODULE).BuildDate=$(shell date)'" ./cmd/d2vm
