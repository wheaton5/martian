#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

PUBLIC_GOBINS=mrc mrf mrg mrp mrs mrt_helper mrjob mrstat
PRIVATE_GOBINS=marsoc mre mrv kepler sere houston redstone rsincoming websoc ligo_server ligo_uploader
GOBINS=$(PUBLIC_GOBINS) $(PRIVATE_GOBINS)
GOLIBTESTS=$(addprefix test-, core util syntax adapter)
GOBINTESTS=$(addprefix test-, $(GOBINS))
GOTESTS=$(GOLIBTESTS) $(GOBINTESTS)
VERSION=$(shell git describe --tags --always --dirty)
RELEASE=false
GO_FLAGS=-ldflags "-X martian/util.__VERSION__='$(VERSION)' -X martian/util.__RELEASE__='$(RELEASE)'"

MARTIAN_PUBLIC=src/vendor/github.com/10XDev/martian-public

export GOPATH=$(shell pwd):$(abspath $(MARTIAN_PUBLIC))

.PHONY: $(GOBINS) grammar houston_web marsoc_web sere_web kepler_web $(GOTESTS) jobmanagers adapters

# Default rule to make it easier to git pull deploy for now.
# Remove this when we switch to package deployment.
marsoc-deploy: marsoc mrjob ligo_uploader

#
# Targets for development builds.
#
all: grammar $(GOBINS) web test

grammar:
	make -C $(MARTIAN_PUBLIC) grammar

$(PUBLIC_GOBINS): jobmanagers adapters
	go install $(GO_FLAGS) martian/cmd/$@ && install -D $(MARTIAN_PUBLIC)/bin/$@ bin/$@

$(PRIVATE_GOBINS): jobmanagers adapters
	go install $(GO_FLAGS) martian/cmd/$@

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

MARTIAN_WEB_PRIVATE=web/mrv/client/editor.coffee web/mrv/client/editor.js \
					web/mrv/client/mrv.coffee web/mrv/client/mrv.js \
					web/mrv/res/css/editor.css \
					web/mrv/templates/editor.html web/mrv/templates/editor.jade \
					web/mrv/templates/mrv.html web/mrv/templates/mrv.jade

martian_web: $(MARTIAN_WEB_PUBLIC)
	(cd web/martian && npm install --no-save && gulp)

mrv_web: martian_web $(MARTIAN_WEB_PRIVATE)
	(cd web/martian && npm install --no-save && gulp)

marsoc_web: martian_web
	(cd web/marsoc && npm install --no-save && gulp)

houston_web: martian_web marsoc_web
	make -C web/houston

kepler_web: martian_web
	(cd web/kepler && npm install --no-save && gulp)

sere_web: martian_web
	(cd web/sere && npm install --no-save && gulp)

web: martian_web mrv_web marsoc_web houston_web kepler_web sere_web

mrt: mrt_helper
	install scripts/mrt bin/mrt

$(GOLIBTESTS): test-%:
	go test -v martian/$*

$(GOBINTESTS): test-%:
	go test -v martian/cmd/$*

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

sake-martian: mrc mre mrf mrg mrp mrjob mrs mrstat mrt mrt_helper ligo_uploader redstone martian_web tools sake-strip sake-martian-strip

sake-test-martian: test

sake-martian-cs: RELEASE=true
sake-martian-cs: sake-martian sake-martian-cs-strip

sake-test-martian-cs: test

sake-marsoc: marsoc mrc mrp mrjob marsoc_web sake-strip

sake-test-marsoc: test

sake-strip:
	# Move martian web files out of src.
	rm -f web/martian
	mv $(MARTIAN_PUBLIC)/web/martian web/martian

	# Strip web dev files.
	rm -f web/*/gulpfile.js
	rm -f web/*/package.json
	rm -f web/*/client/*.coffee
	rm -f web/*/templates/*.jade
	rm -f web/*/templates/*.pug
	rm -f web/*/package-lock.json
	rm -rf web/*/node_modules

	# Remove build intermediates and dev-only files.
	rm -rf pkg
	rm -rf src
	rm -rf scripts
	rm -rf test
	rm -f Makefile
	rm -f bin/goyacc
	rm -f .travis.*
	rm -rf $(YACC_SRC)
	find -type f -name .gitignore -delete
	find -type f -name README.md -delete

sake-martian-strip:
	# Strip marsoc.
	rm -rf web/marsoc
	rm -rf web/mrv
	rm -rf web/kepler
	rm -rf web/sere
	rm -rf web/houston
	rm -rf web/martian/client
	rm -rf web/martian/res
	rm -rf web/martian/build

sake-martian-cs-strip:
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

	#Remove any other binaries which may have been created.
	rm -f bin/marsoc bin/mre bin/mrv bin/kepler bin/sere \
	      bin/houston bin/rsincoming bin/websoc bin/ligo_server
