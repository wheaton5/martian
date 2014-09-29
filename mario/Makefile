#
# Copyright (c) 2014 10X Technologies, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

EXECUTABLES = marsoc marstat mrc mre mrf mrp mrs

VERSION = $(shell git describe --tags --always --dirty)

.PHONY: all $(EXECUTABLES) grammar hoist

all: $(EXECUTABLES) hoist

grammar:
	go tool yacc -p "mm" -o core/grammar.go core/grammar.y && rm y.output

hoist:
	rm -f $(GOPATH)/adapters
	rm -f $(GOPATH)/web
	rm -f $(GOPATH)/web-marsoc
	ln -s src/mario/adapters $(GOPATH)/adapters
	ln -s src/mario/web $(GOPATH)/web
	ln -s src/mario/web-marsoc $(GOPATH)/web-marsoc
	
$(EXECUTABLES):	
	go install -ldflags "-X mario/core.__VERSION__ $(VERSION)" mario/$@