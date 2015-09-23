##################################################
#
# Optional Makefile for convenience purposes and 
# cross-compiling
#
#
GO = go
NPM = npm
PLATFORMS = linux freebsd darwin
ARCHS = amd64

NAME = $(shell basename `pwd`)
DEPENDENCIES = $(shell egrep -r -o -e "\"(github.com|code.google.com)/.+\"" . | cut -d ":" -f2 | sort -u | grep -v $(NAME))
SRC = $(wildcard *.go)
PKGS = $(shell ls -1d */ */*/)
ALLBINS = $(foreach P,$(PLATFORMS),$(foreach A,$(ARCHS), $(NAME).$P.$A))

build: test $(ALLBINS)
	$(GO) build -v

define BUILD
$$(NAME).$1.$2: $$(SRC)
	GOOS=$1 GOARCH=$2 $$(GO) build -v -o $$@
endef
$(foreach P,$(PLATFORMS),$(foreach A,$(ARCHS), $(eval $(call BUILD,$P,$A))))

deps: node_modules
	$(GO) get -u $(DEPENDENCIES)

test:
	for d in $(PKGS); do cd $$d && $(GO) test -v; cd -; done
	$(GO) test -v

clean:
	$(GO) $@
	rm $(ALLBINS) || true


node_modules:
	$(NPM) install jquery 
	$(NPM) install request

