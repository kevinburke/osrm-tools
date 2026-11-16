# osrm-tools Makefile
#
# Set REGION_CONFIG to your region.toml path:
#   make download REGION_CONFIG=examples/region.toml

REGION_CONFIG ?= examples/region.toml
export CONTAINER_ENGINE ?= docker

.PHONY: download get-boundary clip rebuild start stop restart status logs test build fmt lint

# Download OSM data for the region
download:
	bash scripts/download-osm-data.sh $(REGION_CONFIG)

# Download boundary polygon from OpenStreetMap
get-boundary:
	bash scripts/get-boundary-polygon.sh $(REGION_CONFIG)

# Clip OSM data to region boundary (requires osmium-tool)
clip:
	bash scripts/clip-osm-data.sh $(REGION_CONFIG)

# Full pipeline: extract + partition + customize + start server
rebuild:
	bash scripts/rebuild-osrm-data.sh $(REGION_CONFIG)

# Server management
start:
	bash scripts/start-server.sh $(REGION_CONFIG)

stop:
	bash scripts/stop-server.sh

restart: stop start

status:
	$(CONTAINER_ENGINE) ps --filter name=osrm-bike

logs:
	$(CONTAINER_ENGINE) logs --follow osrm-bike

# Go development
test:
	bash scripts/toml-get_test.sh
	bash scripts/osrm-image_test.sh
	go test -trimpath -race ./...

build:
	mkdir -p tmp
	go build -trimpath -o tmp/ ./...

fmt:
	go fmt ./...
	go tool goimports -w .

lint:
	go vet -trimpath ./...
	go tool staticcheck ./...
	shellcheck --exclude=SC1091 scripts/*.sh

install-tools:
	brew install osmium-tool shellcheck

# Tools are pinned in go.mod and installed by `go tool` on demand.
.PHONY: ci check-generated lint
ci: check-generated lint test build

check-generated:
	go tool differ go mod tidy
	go tool differ go fmt ./...
	go tool differ go tool goimports -w .
	go tool differ go generate ./...
