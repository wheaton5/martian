#
# Copyright (c) 2014 10X Genomics, Inc. All rights reserved.
#
# Build a Go package with git version embedding.
#

GOBINS=marsoc mrc mre mrf mrg mrp mrs mrv kepler sere houston redstone
GOTESTS=$(addprefix test-, $(GOBINS) core)
VERSION=$(shell git describe --tags --always --dirty)
RELEASE=false

export GOPATH=$(shell pwd)

.PHONY: $(GOBINS) grammar web $(GOTESTS)

# Default rule to make it easier to git pull deploy for now.
# Remove this when we switch to package deployment.
marsoc-deploy: marsoc

#
# Targets for development builds.
#
all: grammar $(GOBINS) web test

grammar:
	go tool yacc -p "mm" -o src/martian/core/grammar.go src/martian/core/grammar.y && rm y.output

$(GOBINS):
	go install -ldflags "-X martian/core.__VERSION__=$(VERSION) -X martian/core.__RELEASE__=$(RELEASE)" martian/$@

web:
	cd web/martian; npm install; gulp; cd $(GOPATH)
	cd web/marsoc; npm install; gulp; cd $(GOPATH)
	cd web/kepler; npm install; gulp; cd $(GOPATH)
	cd web/sere; npm install; gulp; cd $(GOPATH)
	cd web/houston; npm install; gulp; cd $(GOPATH)

$(GOTESTS): test-%:
	go test -v martian/$*

test: $(GOTESTS)

clean:
	rm -rf $(GOPATH)/bin
	rm -rf $(GOPATH)/pkg

#
# Targets for Sake builds.
#
ifdef SAKE_VERSION
VERSION=$(SAKE_VERSION)
endif

sake-martian: mrc mre mrf mrg mrp mrs sake-strip sake-martian-strip

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
	rm -f Makefile
	rm -f README.md

sake-martian-strip:
	# Strip marsoc.
	rm -rf web/marsoc
	rm -rf web/kepler
	rm -rf web/sere
	rm -rf web/houston

sake-martian-cs-strip:
	# Remove mrv template.
	rm web/martian/templates/mrv.html
	
	# Remove pd job templates.
	rm -f jobmanagers/*.template
