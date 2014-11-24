#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

EXECUTABLES = marsoc marstat mrc mre mrf mrg mrp mrs mrv
TESTABLES = marsoc marstat core mrc mre mrf mrg mrp mrs mrv

VERSION = $(shell git describe --tags --always --dirty)

.PHONY: all $(EXECUTABLES) test grammar web hoist 

marsoc-deploy: marsoc hoist

all: grammar $(EXECUTABLES) web hoist test

test:
	for testable in $(TESTABLES) ; do \
		echo [test - $$testable] ; \
		go test mario/$$testable ; \
	done

grammar:
	@echo [$@]
	go tool yacc -p "mm" -o core/grammar.go core/grammar.y && rm y.output

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
	cd web; gulp; cd ..
	cd web-marsoc; gulp; cd ..
	
$(EXECUTABLES):
	@echo [bin - $@]
	go install -ldflags "-X mario/core.__VERSION__ $(VERSION)" mario/$@

