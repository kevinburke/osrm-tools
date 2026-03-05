# osrm-tools Makefile
#
# Set REGION_CONFIG to your region.toml path:
#   make download REGION_CONFIG=examples/region.toml

REGION_CONFIG ?= examples/region.toml

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
	docker ps | grep osrm-bike || echo "Server not running"

logs:
	docker logs -f osrm-bike

# Go development
test:
	GO111MODULE=on go test -trimpath ./...

build:
	GO111MODULE=on go build -trimpath ./...

fmt:
	GO111MODULE=on go fmt ./...
	goimports -w .

lint:
	GO111MODULE=on go vet ./...
	shellcheck --exclude=SC1091 scripts/*.sh

install-tools:
	brew install osmium-tool shellcheck
