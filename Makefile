#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

EXECUTABLES = marsoc marstat mrc mre mrf mrg mrp mrs mrv
TESTABLES := $(addprefix test-,$(EXECUTABLES) core)

VERSION = $(shell git describe --tags --always --dirty)

export GOPATH=$(shell pwd)

.PHONY: all $(EXECUTABLES) grammar web $(TESTABLES) test

# Default rule to make it easier to git pull deploy for now.
# Remove this when we switch to package deployment.
marsoc-deploy: marsoc 

#
# Targets for development builds.
# 
all: grammar $(EXECUTABLES) web test

grammar:
	@echo [$@]
	go tool yacc -p "mm" -o src/mario/core/grammar.go src/mario/core/grammar.y && rm y.output

$(EXECUTABLES):
	@echo [bin - $@]
	go install -ldflags "-X mario/core.__VERSION__ $(VERSION)" mario/$@

web:
	@echo [$@]
	cd web/mario; gulp; cd $(GOPATH)
	cd web/marsoc; gulp; cd $(GOPATH)

$(TESTABLES): test-%:
	go test mario/$*

test: $(TESTABLES)

#
# Targets for Sake builds.
# 
ifdef SAKE_VERSION
VERSION = $(SAKE_VERSION)
endif

sake-mario: mrc mre mrf mrg mrp mrs sake-mario-strip

sake-marsoc: marsoc mrc mrp sake-mario-web sake-marsoc-web

sake-mario-strip:
	# Strip dev files.
	rm -f web/mario/gulpfile.js
	rm -f web/mario/package.json
	rm -f web/mario/client/*.coffee
	rm -f web/mario/templates/*.jade

	# Strip marsoc.
	rm -rf web/marsoc

	# Remove build intermediates and source code. 
	rm -rf pkg
	rm -rf src

sake-marsoc-strip:
	# Strip dev files.
	rm -f web-marsoc/gulpfile.js
	rm -f web-marsoc/package.json
	rm -f web-marsoc/client/*.coffee
	rm -f web-marsoc/templates/*.jade
