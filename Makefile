#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

GOBINS=marsoc mrc mre mrf mrg mrp mrs mrv kepler sere houston redstone rsincoming websoc mrt_helper ligo/ligo_server ligo/ligo_uploader
GOTESTS=$(addprefix test-, $(GOBINS) core)
VERSION=$(shell git describe --tags --always --dirty)
RELEASE=false
GO_VERSION=$(strip $(shell go version | sed 's/.*go\([0-9]*\.[0-9]*\).*/\1/' | tr -d " "))

# Older versions of Go use the "-X foo bar" syntax.  Newer versions either warn
# or error on that syntax, and use "-X foo=bar" instead.
LINK_SEPERATOR=$(if $(filter 1.5, $(word 1, $(sort 1.5 $(GO_VERSION)))),=, )
GO_FLAGS=-ldflags "-X martian/core.__VERSION__$(LINK_SEPERATOR)'$(VERSION)' -X martian/core.__RELEASE__$(LINK_SEPERATOR)'$(RELEASE)'"

export GOPATH=$(shell pwd)

.PHONY: $(GOBINS) grammar web $(GOTESTS)

# Default rule to make it easier to git pull deploy for now.
# Remove this when we switch to package deployment.
marsoc-deploy: marsoc ligo/ligo_uploader

#
# Targets for development builds.
#
all: grammar $(GOBINS) web test

# Go 1.8 removed yacc from the base distribution and moved it to the tools repo.
YACC_SRC=src/github.com/golang/tools/cmd/goyacc/yacc.go
$(YACC_SRC):
	go get github.com/golang/tools/cmd/goyacc
bin/goyacc: $(YACC_SRC)
	go install github.com/golang/tools/cmd/goyacc
YACCDEP=$(if $(filter 1.8, $(word 1, $(sort 1.8 $(GO_VERSION)))),bin/goyacc,)
YACC_TOOL=$(if $(filter 1.8, $(word 1, $(sort 1.8 $(GO_VERSION)))),bin/goyacc,go tool yacc)

src/martian/core/grammar.go: $(YACCDEP) src/martian/core/grammar.y
	$(YACC_TOOL) -p "mm" -o src/martian/core/grammar.go src/martian/core/grammar.y && rm y.output

grammar: src/martian/core/grammar.go

$(GOBINS):
	go install $(GO_FLAGS) martian/$@

web:
	(cd web/martian && npm install && gulp)
	(cd web/marsoc && npm install && gulp)
	(cd web/kepler && npm install && gulp)
	(cd web/sere && npm install && gulp)
	make -C web/houston

mrt:
	cp scripts/mrt bin/mrt

$(GOTESTS): test-%:
	go test -v martian/$*

test: $(GOTESTS)

clean:
	rm -rf $(GOPATH)/bin
	rm -rf $(GOPATH)/pkg
	make -C web/houston clean

#
# Targets for Sake builds.
#
ifdef SAKE_VERSION
VERSION=$(SAKE_VERSION)
endif

sake-martian: mrc mre mrf mrg mrp mrs mrt mrt_helper ligo/ligo_uploader redstone sake-strip sake-martian-strip

sake-test-martian: test

sake-martian-cs: RELEASE=true
sake-martian-cs: sake-martian sake-martian-cs-strip

sake-test-martian-cs: test

sake-marsoc: marsoc mrc mrp sake-strip

sake-test-marsoc: test

sake-strip:
	# Strip web dev files.
	rm -f web/*/gulpfile.js
	rm -f web/*/package.json
	rm -f web/*/client/*.coffee
	rm -f web/*/templates/*.jade

	# Remove build intermediates and dev-only files.
	rm -rf pkg
	rm -rf src
	rm -rf scripts
	rm -rf test
	rm -f Makefile
	rm -f README.md
	rm -f bin/goyacc
	rm -rf $(YACC_SRC)

sake-martian-strip:
	# Strip marsoc.
	rm -rf web/marsoc
	rm -rf web/kepler
	rm -rf web/sere
	rm -rf web/houston

sake-martian-cs-strip:
	# Remove mrv assets.
	rm web/martian/client/mrv.js
	rm web/martian/templates/mrv.html

	# Remove pd job templates.
	rm -f jobmanagers/*.template

	# Remove hydra-specific stuff.
	rm -f jobmanagers/hydra_queue.py

	# Remove ligo_uploader
	rm -f bin/ligo_uploader

	# Remove mrt
	rm -f bin/mrt*

	# Remove mrg
	rm -f bin/mrg
