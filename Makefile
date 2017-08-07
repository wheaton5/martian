#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

PUBLIC_GOBINS=mrc mrf mrg mrp mrs mrt_helper mrjob mrstat
PRIVATE_GOBINS=marsoc mre mrv kepler sere houston redstone rsincoming websoc ligo/ligo_server ligo/ligo_uploader
GOBINS=$(PUBLIC_GOBINS) $(PRIVATE_GOBINS)
GOTESTS=$(addprefix test-, $(GOBINS) core)
VERSION=$(shell git describe --tags --always --dirty)
RELEASE=false
GO_FLAGS=-ldflags "-X martian/core.__VERSION__='$(VERSION)' -X martian/core.__RELEASE__='$(RELEASE)'"

MARTIAN_PUBLIC=src/vendor/github.com/10XDev/martian-public

export GOPATH=$(shell pwd):$(abspath $(MARTIAN_PUBLIC))

.PHONY: $(GOBINS) grammar houston_web marsoc_web sere_web kepler_web $(GOTESTS) jobmanagers adapters

# Default rule to make it easier to git pull deploy for now.
# Remove this when we switch to package deployment.
marsoc-deploy: marsoc mrjob ligo/ligo_uploader

#
# Targets for development builds.
#
all: grammar $(GOBINS) web test

grammar:
	make -C $(MARTIAN_PUBLIC) grammar

$(PUBLIC_GOBINS): jobmanagers adapters
	go install $(GO_FLAGS) martian/$@ && install -D $(MARTIAN_PUBLIC)/bin/$@ bin/$@

$(PRIVATE_GOBINS): jobmanagers adapters
	go install $(GO_FLAGS) martian/$@

# Target to pull latest martian public branch.
latest-public:
	cd $(MARTIAN_PUBLIC) && git pull origin master && git submodule update --init --recursive

JOBMANAGER_PUBLIC=$(shell unset CDPATH && cd $(MARTIAN_PUBLIC)/jobmanagers && ls)

jobmanagers/%: $(MARTIAN_PUBLIC)/jobmanagers/%
	install $< $@

jobmanagers: $(addprefix jobmanagers/, $(JOBMANAGER_PUBLIC))

adapters/%: $(MARTIAN_PUBLIC)/adapters/%
	install -D $< $@

ADAPTER_SRCS=$(shell cd $(MARTIAN_PUBLIC) && find adapters -type f)

adapters: $(ADAPTER_SRCS)

MARTIAN_WEB_PUBLIC=$(shell cd $(MARTIAN_PUBLIC) && find web/martian -type f)

web/martian/%: $(MARTIAN_PUBLIC)/web/martian/%
	install -D $< $@

MARTIAN_WEB_PRIVATE=web/martian/client/editor.coffee web/martian/client/editor.js \
					web/martian/client/mrv.coffee web/martian/client/mrv.js \
					web/martian/res/css/editor.css \
					web/martian/templates/editor.html web/martian/templates/editor.jade \
					web/martian/templates/mrv.html web/martian/templates/mrv.jade

martian_web: $(MARTIAN_WEB_PUBLIC) $(MARTIAN_WEB_PRIVATE)
	(cd web/martian && npm install --no-save && gulp)

marsoc_web: martian_web
	(cd web/marsoc && npm install --no-save && gulp)

houston_web: martian_web marsoc_web
	make -C web/houston

kepler_web: martian_web
	(cd web/kepler && npm install --no-save && gulp)

sere_web: martian_web
	(cd web/sere && npm install --no-save && gulp)

web: martian_web marsoc_web houston_web kepler_web sere_web

mrt: mrt_helper
	install scripts/mrt bin/mrt

$(GOTESTS): test-%:
	go test -v martian/$*

test: $(GOTESTS)

clean:
	rm -rf $(GOPATH)/bin
	rm -rf $(GOPATH)/pkg
	make -C web/houston clean
	make -C $(MARTIAN_PUBLIC) clean

tools: $(MARTIAN_PUBLIC)/tools
	install -d $< $@

#
# Targets for Sake builds.
#
ifdef SAKE_VERSION
VERSION=$(SAKE_VERSION)
endif

sake-martian: mrc mre mrf mrg mrp mrjob mrs mrstat mrt mrt_helper ligo/ligo_uploader redstone web tools sake-strip sake-martian-strip

sake-test-martian: test

sake-martian-cs: RELEASE=true
sake-martian-cs: sake-martian sake-martian-cs-strip

sake-test-martian-cs: test

sake-marsoc: marsoc mrc mrp mrjob martian_web marsoc_web sake-strip

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
	rm -f .travis.*
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
