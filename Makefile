VER ?= `git describe --tags --dirty --always`

help: ## Displays help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-z0-9A-Z_-]+:.*?##/ { printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: test ## Build binaries with version set
	@CGO_ENABLED=0 go build -o kubediff -ldflags "-w -s \
	-X github.com/prometheus/common/version.Version=${VER} \
	-X github.com/prometheus/common/version.Revision=`git rev-parse --short HEAD` \
	-X github.com/prometheus/common/version.Branch=`git rev-parse --abbrev-ref HEAD` \
	-X github.com/prometheus/common/version.BuildUser=${USER}@`hostname` \
	-X github.com/prometheus/common/version.BuildDate=`date +%Y/%m/%d-%H:%M:%SZ`" \
	./cmd/kubediff

test: ## Run tests
	@go vet ./...
	@go test ./...

tag: ## Create a new git tag with the new version
	@v=$$(echo ${VER} | sed -r 's/([^-]+)(-.+)?/\1/') && \
	IFS='.' read -r a b c <<< "$$v" && \
	c=$$((c + 1)) && v="$$a.$$b.$$c" && \
	echo "Creating new tag: $$v" && \
	git tag -a $$v -m "Release $$v" && \
	git push origin $$v
