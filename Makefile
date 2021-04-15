PACKAGE = github.com/dfinlab/meter

FAKE_GOPATH_SUFFIX = $(shell [ -e ".fake_gopath_suffix" ] || date +%s > .fake_gopath_suffix; cat .fake_gopath_suffix)
FAKE_GOPATH = /tmp/meter-build-$(FAKE_GOPATH_SUFFIX)
export GOPATH = $(FAKE_GOPATH)

SRC_BASE = $(FAKE_GOPATH)/src/$(PACKAGE)

GIT_COMMIT = $(shell git --no-pager log --pretty="%h" -n 1)
GIT_TAG = $(shell git tag -l --points-at HEAD)
METER_VERSION = $(shell cat cmd/meter/VERSION)
DISCO_VERSION = $(shell cat cmd/disco/VERSION)

PACKAGES = `cd $(SRC_BASE) && go list ./... | grep -v '/vendor/'`

.PHONY: meter disco probe all clean test

meter: |$(SRC_BASE)
	@echo "building $@..."
	@cd $(SRC_BASE) && go build -v -i -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(METER_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/meter
	@echo "done. executable created at 'bin/$@'"

disco: |$(SRC_BASE)
	@echo "building $@..."
	@cd $(SRC_BASE) && go build -v -i -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(DISCO_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/disco
	@echo "done. executable created at 'bin/$@'"

try: |$(SRC_BASE)
	@echo "building $@..."
	@cd $(SRC_BASE) && go build -v -i -o $(CURDIR)/bin/$@ -ldflags "-X main.version=$(METER_VERSION) -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)" ./cmd/try
	@echo "done. executable created at 'bin/$@'"



dep: |$(SRC_BASE)
ifeq ($(shell command -v dep 2> /dev/null),)
	@git submodule update --init
else
	@cd $(SRC_BASE) && dep ensure -vendor-only
endif

$(SRC_BASE):
	@mkdir -p $(dir $@)
	@ln -sf $(CURDIR) $@

all: meter disco

clean:
	-rm -rf \
$(FAKE_GOPATH) \
$(CURDIR)/bin/meter \
$(CURDIR)/bin/disco 

test: |$(SRC_BASE)
	@cd $(SRC_BASE) && go test -cover $(PACKAGES)

