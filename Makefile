.PHONY: all build clean help version

.DEFAULT_GOAL: build

#
# Variables
#

ARCHITECTURES=386 amd64
BINARY=wikiipsum
PLATFORMS=darwin linux windows
VERSION = $(shell \
		git -C . describe --tags 2> /dev/null || \
		git -C . rev-parse --short HEAD 2> /dev/null || \
		echo "unknown" \
	)

#
# Targets
#

clean:   ## Remove binary files.
	rm -fv $(BINARY)*

## Compile binaries for all OS.
all:
	$(foreach GOOS, $(PLATFORMS), $(foreach GOARCH, $(ARCHITECTURES), $(shell export GOOS=$(GOOS); export GOARCH=$(GOARCH); go build -o $(BINARY)-$(VERSION)-$(GOOS)-$(GOARCH) -ldflags="-X 'main.Version=$(VERSION)'")))

build:   ## Compile a binary.
	go build -o $(BINARY) -ldflags="-X 'main.Version=$(VERSION)'"

help:    ## Show help.
	@echo
	@echo '  Usage:'
	@echo '	make <target>'
	@echo
	@echo '  Targets:'
	@fgrep -h "##" $(MAKEFILE_LIST) | fgrep -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'
	@echo

version: ## Print version.
	@echo $(VERSION)
