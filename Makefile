##################################################
#
# Optional Makefile for convenience purposes and 
# cross-compiling
#
#
GO = go
PLATFORMS = linux freebsd darwin
ARCHS = amd64
EDITOR = gvim

NAME = $(shell basename `pwd`)
DEPENDENCIES = $(shell egrep -r -o -e "\"(github.com|code.google.com)/.+\"" . | cut -d ":" -f2 | sort -u | grep -v $(NAME))
SRC = $(wildcard *.go */*.go */*/*.go)
PKGS = $(shell ls -1d */ */*/)
ALLBINS = $(foreach P,$(PLATFORMS),$(foreach A,$(ARCHS), $(NAME).$P.$A))
VERSIONTAG = `git describe --always` `date +%Y%m%d`
CGO_FLAGS =
GOENV = CC=gcc $(CGO_FLAGS)
.PHONY: build
build: test $(ALLBINS)
	$(GO) build -v

define BUILD
$$(NAME).$1.$2: $$(SRC)
	GOOS=$1 GOARCH=$2 $$(GO) build -v -o $$@
endef
$(foreach P,$(PLATFORMS),$(foreach A,$(ARCHS), $(eval $(call BUILD,$P,$A))))

tags:
	$(GOTAGS) gotags -R ../ > tags

.PHONY: goclean
goclean:
	$(GO) clean
	rm $(ALLBINS) || true

.PHONY: godeps
godeps:
	$(GO) get -u $(DEPENDENCIES)

.PHONY: edit
edit: tags
	$(EDITOR) $(SRC)


# end of base Makefile
#############################################

SRC+=Makefile
NPM = npm

.PHONY: deps
deps: node_modules godeps

.PHONY: test
test:
	for d in $(PKGS); do cd $$d && $(GO) test -v; cd -; done
	$(GO) test -v

.PHONY: clean
clean: goclean

.PHONY: fullclean
fullclean: clean
	rm -rf node_modules

node_modules:
	$(NPM) install jquery 
	$(NPM) install request

