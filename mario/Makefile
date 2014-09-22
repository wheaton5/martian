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
	@echo "Building $@."
	go install -ldflags "-X main.__VERSION__ $(VERSION)" mario/$@