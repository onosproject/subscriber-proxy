# SPDX-FileCopyrightText: 2019-present Open Networking Foundation <info@opennetworking.org>
#
# SPDX-License-Identifier: Apache-2.0

export CGO_ENABLED=1
export GO111MODULE=on

.PHONY: build

build: # @HELP build the Go binaries and run all validations (default)
build: ;

build-tools:=$(shell if [ ! -d "./build/build-tools" ]; then mkdir build && cd build && git clone https://github.com/onosproject/build-tools.git; fi)
include ./build/build-tools/make/onf-common.mk

test: # @HELP run the unit tests and source code validation
test: build license_check_apache

jenkins-test:  # @HELP run the unit tests and source code validation producing a junit style report for Jenkins
jenkins-test: build license_check_apache

publish: # @HELP publish version on github and dockerhub
	./build/build-tools/publish-version ${VERSION} onosproject/subscriber-proxy

jenkins-publish: ; # @HELP Jenkins calls this to publish artifacts

clean:: ; # @HELP remove all the build artifacts

