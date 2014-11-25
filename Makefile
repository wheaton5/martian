#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

EXECUTABLES = marsoc marstat mrc mre mrf mrg mrp mrs mrv
TESTABLES = marsoc marstat core mrc mre mrf mrg mrp mrs mrv
TESTRULES := $(addprefix test-,$(TESTABLES))

VERSION = $(shell git describe --tags --always --dirty)

.PHONY: all $(EXECUTABLES) test grammar web hoist ${test-executables}

marsoc-deploy: marsoc hoist

all: grammar test $(EXECUTABLES) web hoist 

test: $(TESTRULES)
$(TESTRULES): test-%:
	go test mario/$*

grammar:
	@echo [$@]
	go tool yacc -p "mm" -o mario/core/grammar.go mario/core/grammar.y && rm y.output

hoist:
	@echo [$@]
	rm -f $(GOPATH)/adapters
	rm -f $(GOPATH)/web
	rm -f $(GOPATH)/web-marsoc
	ln -s src/mario/adapters $(GOPATH)/adapters
	ln -s src/mario/web $(GOPATH)/web
	ln -s src/mario/web-marsoc $(GOPATH)/web-marsoc

web:
	@echo [$@]
	cd mario/web; gulp; cd $(GOPATH)/src
	cd mario/web-marsoc; gulp; cd $(GOPATH)/src
	
$(EXECUTABLES):
	@echo [bin - $@]
	go install -ldflags "-X mario/core.__VERSION__ $(VERSION)" mario/$@

ifdef SAKE_VERSION
VERSION = $(SAKE_VERSION)
endif
sake-build: mrc mre mrf mrg mrp mrs sake-finish

sake-finish:
	# Hoist adapters
	rm -rf $(GOPATH)/adapters
	cp -rf adapters $(GOPATH)

	# Hoist web; remove dev files
	rm -rf $(GOPATH)/web
	cp -rf web $(GOPATH)
	rm -f $(GOPATH)/web/gulpfile.js
	rm -f $(GOPATH)/web/package.json
	rm -f $(GOPATH)/web/client/*.coffee
	rm -f $(GOPATH)/web/templates/*.jade

	# Hoist .version
	cp -f .version $(GOPATH)/.version

	# Remove source code and build intermediates
	rm -rf $(GOPATH)/pkg
	rm -rf $(GOPATH)/src
