# Copyright 2016 CDW.  #
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

## Thank you to Tim Hockin and the Kubernetes team!
## https://github.com/thockin/go-build-template

# The project root (used with import path, ldflags, etc)
PROJECT := cdwlabs/armor

# The binary to build (just the basename).
BIN := armor

# This repo's root import path (under GOPATH).
PKG := github.com/$(PROJECT)

# Where to push the docker image (e.g. docker.io, gcr.io)
DOCKER_REGISTRY ?= docker.io

# Docker image owner
IMAGE_OWNER ?= pinterb

# Which architecture to build - see $(ALL_ARCH) for options.
ARCH ?= amd64

# This version-strategy uses git tags to set the version string
VERSION := $(shell git describe --always --dirty)
#
# This version-strategy uses a manual value to set the version string
#VERSION := 1.2.3

###
### These variables should not need tweaking.
###

SRC_DIRS := pb pkg cmd # directories which hold app source (not vendored)

ALL_ARCH := amd64 arm arm64 ppc64le

# Set default base image dynamically for each arch
ifeq ($(ARCH),amd64)
   	BASEIMAGE?=alpine
endif
ifeq ($(ARCH),arm)
    BASEIMAGE?=armel/busybox
endif
ifeq ($(ARCH),arm64)
    BASEIMAGE?=aarch64/busybox
endif
ifeq ($(ARCH),ppc64le)
    BASEIMAGE?=ppc64le/busybox
endif

ifeq ($(DOCKER_REGISTRY),docker.io)
  IMAGE = $(IMAGE_OWNER)/$(BIN)-$(ARCH)
else
  IMAGE = $(DOCKERK_REGISTRY)/$(IMAGE_OWNER)/$(BIN)-$(ARCH)
endif

BUILD_IMAGE ?= golang:1.7-alpine

# If you want to build all binaries, see the 'all-build' rule.
# If you want to build all containers, see the 'all-container' rule.
# If you want to build AND push all containers, see the 'all-push' rule.
all: build

build-%:
	@$(MAKE) --no-print-directory ARCH=$* build

container-%:
	@$(MAKE) --no-print-directory ARCH=$* container

push-%:
	@$(MAKE) --no-print-directory ARCH=$* push

all-build: $(addprefix build-, $(ALL_ARCH))

all-container: $(addprefix container-, $(ALL_ARCH))

all-push: $(addprefix push-, $(ALL_ARCH))

build: bin/$(ARCH)/$(BIN)  ## Compiles your app.

bin/$(ARCH)/$(BIN): build-dirs
	@echo "building: $@"
	@docker run                                                            \
	    -ti                                                                \
	    -u $$(id -u):$$(id -g)                                             \
	    -v $$(pwd)/.go:/go                                                 \
	    -v $$(pwd):/go/src/$(PKG)                                          \
	    -v $$(pwd)/bin/$(ARCH):/go/bin                                     \
	    -v $$(pwd)/bin/$(ARCH):/go/bin/linux_$(ARCH)                       \
	    -v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static  \
	    -w /go/src/$(PKG)                                                  \
	    $(BUILD_IMAGE)                                                     \
	    /bin/sh -c "                                                       \
	        ARCH=$(ARCH)                                                   \
	        VERSION=$(VERSION)                                             \
	        PKG=$(PKG)                                                     \
	        ./build/build.sh                                               \
	    "

DOTFILE_IMAGE = $(subst /,_,$(IMAGE))-$(VERSION)

container: .container-$(DOTFILE_IMAGE) container-name ## Builds the container image. It will calculate the image tag based on the most recent git tag, and whether the repo is "dirty".
.container-$(DOTFILE_IMAGE): bin/$(ARCH)/$(BIN) Dockerfile.in
	@sed \
	    -e 's|ARG_BIN|$(BIN)|g' \
	    -e 's|ARG_ARCH|$(ARCH)|g' \
	    -e 's|ARG_FROM|$(BASEIMAGE)|g' \
	    Dockerfile.in > .dockerfile-$(ARCH)
	@docker build -t $(IMAGE):$(VERSION) -f .dockerfile-$(ARCH) .
	@docker images -q $(IMAGE):$(VERSION) > $@

container-name:
	@echo "container: $(IMAGE):$(VERSION)"

push: .push-$(DOTFILE_IMAGE) push-name  ## Pushes the container image to DOCKER_REGISTRY
.push-$(DOTFILE_IMAGE): .container-$(DOTFILE_IMAGE)
	$(DOCKER_PUSH) $(IMAGE):$(VERSION)
	@docker images -q $(IMAGE):$(VERSION) > $@

push-name:
	@echo "pushed: $(IMAGE):$(VERSION)"

version: ## Display various versions
	@echo "THock ver:       ${VERSION}"
	@echo "Build tag:       ${DOCKER_VERSION}"
	@echo "Image Registry:  ${DOCKER_REGISTRY}"

#	    -u $$(id -u):$$(id -g)                                             \

test: build-dirs
	@docker run                                                            \
	    -ti                                                                \
	    -v $$(pwd)/.go:/go                                                 \
	    -v $$(pwd):/go/src/$(PKG)                                          \
	    -v $$(pwd)/bin/$(ARCH):/go/bin                                     \
	    -v $$(pwd)/.go/std/$(ARCH):/usr/local/go/pkg/linux_$(ARCH)_static  \
			-v /var/run/docker.sock:/var/run/docker.sock                       \
	    -w /go/src/$(PKG)                                                  \
	    --net=host                                                         \
	    $(BUILD_IMAGE)                                                     \
	    /bin/sh -c "                                                       \
	        ./build/test.sh $(SRC_DIRS)                                    \
	    "

build-dirs:
	@mkdir -p bin/$(ARCH)
	@mkdir -p .go/src/$(PKG) .go/pkg .go/bin .go/std/$(ARCH)

clean: container-clean bin-clean

container-clean: container-clean-untagged
	rm -rf .container-* .dockerfile-* .push-*

bin-clean:
	rm -rf .go bin

## Additions by pinterb

vendor-clean:
	rm -rf vendor/github.com/Sirupsen/logrus/examples
	rm -rf vendor/github.com/apache/thrift/lib/go/test
	rm -rf vendor/github.com/apache/thrift/test
	rm -rf vendor/github.com/apache/thrift/tutorial
	rm -rf vendor/github.com/bugsnag/bugsnag-go/examples
	rm -rf vendor/github.com/go-kit/kit/examples
	rm -rf vendor/github.com/rakyll/statik/example
	rm -rf vendor/github.com/docker/distribution/registry/storage/driver/testsuites
	rm -rf vendor/github.com/influxdata/influxdb/services/collectd/test_client

.PHONY: container-clean-untagged
container-clean-untagged: container-clean-stopped
	docker images --no-trunc | grep none | awk '{print $$3}' | xargs -r docker rmi

.PHONY: container-clean-stopped
container-clean-stopped:
	for i in `docker ps --no-trunc -a -q`;do docker rm $$i;done

HAS_GLIDE := $(shell command -v glide;)
HAS_GLIDE_VC := $(shell command -v glide-vc;)
HAS_HG := $(shell command -v hg;)
HAS_GIT := $(shell command -v git;)

GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_SHA := $(shell git rev-parse --short HEAD)
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null)
GIT_DIRTY = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")

TAGS      :=
GO        ?= go
TESTS     := .
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=
BINDIR    := $(CURDIR)/bin

ifdef VERSION
  DOCKER_VERSION = $(VERSION)
  BINARY_VERSION = $(VERSION)
endif

DOCKER_VERSION ?= git-${GIT_SHA}
BINARY_VERSION ?= ${GIT_TAG}-${GIT_SHA}

LDFLAGS :=
LDFLAGS += -X ${PROJECT}/pkg/version.Version=${GIT_TAG}
LDFLAGS += -X ${PROJECT}/pkg/version.GitCommit=${GIT_COMMIT}
LDFLAGS += -X ${PROJECT}/pkg/version.GitTreeState=${GIT_DIRTY}

DOCKER_PUSH = docker push
ifeq ($(DOCKER_REGISTRY),gcr.io)
  DOCKER_PUSH = gcloud docker push
endif

.PHONY: bootstrap
bootstrap: ## Boostraps your build by checking prerequisites, vendoring dependencies, etc.
ifndef HAS_GLIDE
		go get -u github.com/Masterminds/glide
endif
ifndef HAS_GLIDE_VC
		go get -u github.com/mitchellh/gox
endif
ifndef HAS_HG
		$(error You must install Mercurial (hg))
endif
ifndef HAS_GIT
		$(error You must install Git)
endif
		glide install --strip-vendor
		@$(MAKE) vendor-clean
		mkdir -p /tmp/armorbuild
		cp -R vendor/github.com/fsouza /tmp/armorbuild/
		cp -R vendor/github.com/docker /tmp/armorbuild/
		glide-vc
		cp -R /tmp/armorbuild/fsouza vendor/github.com/
		cp -R /tmp/armorbuild/docker vendor/github.com/
		rm -rf /tmp/armorbuild

.PHONY: dev-build
dev-build: ## Dev build that should be used during development AND before your /vendor directory is finally populated
	GOBIN=$(BINDIR) $(GO) install $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' '$(PKG)/cmd/...'

.PHONY: dev-test
dev-test: dev-build ## Use this like 'dev-build' ...but for testing.
dev-test: TESTFLAGS += -race -v
dev-test: dev-test-unit
#dev-test: dev-test-style

.PHONY: dev-test-unit
dev-test-unit:
	$(GO) test $(GOFLAGS) -run $(TESTS) $(PKG)/pkg/... $(TESTFLAGS)
	$(GO) test $(GOFLAGS) -run $(TESTS) $(PKG)/cmd/... $(TESTFLAGS)

.PHONY: dev-test-style
dev-test-style:
	@build/validate-go.sh
	@build/validate-license.sh

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

