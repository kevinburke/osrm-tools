module github.com/kevinburke/osrm-tools

go 1.26.0

toolchain go1.27.4

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/kevinburke/handlers v0.50.0
	github.com/paulmach/osm v0.9.0
	github.com/twpayne/go-geos v0.23.0
)

require (
	github.com/DataDog/czlib v0.0.0-20260429183542-0a71558fab14 // indirect
	github.com/gofrs/uuid/v5 v5.5.1 // indirect
	github.com/inconshreveable/log15/v3 v3.2.1 // indirect
	github.com/kevinburke/differ v1.4.0 // indirect
	github.com/kevinburke/rest/v2 v2.15.0 // indirect
	github.com/paulmach/orb v0.14.0 // indirect
	github.com/paulmach/protoscan v0.2.1 // indirect
	go.mongodb.org/mongo-driver/v2 v2.10.0 // indirect
	golang.org/x/exp/typeparams v0.0.0-20231108232855-2478ac86f678 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/telemetry v0.0.0-20261009151841-97e8ea77aa2a // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	honnef.co/go/tools v0.8.2 // indirect
)

tool (
	github.com/kevinburke/differ
	golang.org/x/tools/cmd/goimports
	honnef.co/go/tools/cmd/staticcheck
)
