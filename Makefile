GO_IMAGE := golang:1.26
GO_CACHE := -v $(CURDIR):/src -w /src -v timescale-doctor-gocache:/go
GO := docker run --rm $(GO_CACHE) $(GO_IMAGE)

DEMO_IMAGE := timescale/timescaledb:2.32.0-pg17
DEMO := tsdoctor-demo

test:
	$(GO) sh -c 'go vet ./... && go test ./...'

tidy:
	$(GO) go mod tidy

build:
	$(GO) env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath -o bin/tsdoctor ./cmd/tsdoctor

# Starts a throwaway TimescaleDB, loads testdata/seed.sql, prints the report and removes the
# database. tsdoctor runs in a container on the database's network, so no host port is opened.
# Flags go in ARGS, for example: make demo ARGS=--json
demo: demo-clean
	docker run -d --name $(DEMO) -e POSTGRES_PASSWORD=demo $(DEMO_IMAGE) \
		-c shared_preload_libraries=timescaledb,pg_stat_statements -c timescaledb.telemetry_level=off
	until docker exec $(DEMO) pg_isready -q -h 127.0.0.1 -U postgres; do sleep 1; done
	docker exec -i $(DEMO) psql -q -U postgres -o /dev/null < testdata/seed.sql
	@echo "waiting 10s for the background jobs to run once"
	@sleep 10
	docker run --rm --network container:$(DEMO) $(GO_CACHE) $(GO_IMAGE) \
		go run ./cmd/tsdoctor $(ARGS) "postgres://postgres:demo@localhost:5432/postgres"; \
		status=$$?; docker rm -f $(DEMO) >/dev/null; exit $$status

demo-clean:
	-docker rm -f $(DEMO) 2>/dev/null

.PHONY: test tidy build demo demo-clean
