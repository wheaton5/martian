#
# Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

EXECUTABLES = marsoc marstat mrc mre mrf mrp mrs

VERSION = $(shell git describe --tags --always --dirty)

.PHONY: all grammar $(EXECUTABLES)

all: grammar $(EXECUTABLES)

grammar:
	@echo "Generating parser code."
	go tool yacc -p "mm" -o core/grammar.go core/grammar.y && rm y.output

$(EXECUTABLES):
	rm -f $(GOPATH)/adapters
	rm -f $(GOPATH)/web
	rm -f $(GOPATH)/web-marsoc
	ln -s src/mario/adapters $(GOPATH)/adapters
	ln -s src/mario/web $(GOPATH)/web
	ln -s src/mario/web-marsoc $(GOPATH)/web-marsoc
	@echo "Building $@."
	go install -ldflags "-X mario/core.__VERSION__ $(VERSION)" mario/$@