all: serve

bower_components: bower.json
	bower install

OUTPUT_FILES=index.html service-worker.js manifest.json src/houston-app.html \
						 bower_components/webcomponentsjs/webcomponents-lite.min.js

SOURCES=$(OUTPUT_FILES) polymer.json index.html \
				$(wildcard src/*) $(wildcard *.js)

OUTPUT_DIR=build/bundled

BUILD_OUTPUTS=$(addprefix $(OUTPUT_DIR)/, $(OUTPUT_FILES))

build: bower_components $(SOURCES)
	polymer lint -i $(SOURCES)
	polymer build

$(BUILD_OUTPUTS): build

DESTS=$(addprefix serve/, $(OUTPUT_FILES))

serve/%: $(OUTPUT_DIR)/%
	install -D $^ $@

serve: $(DESTS)

clean:
	rm -rf bower_components build serve
