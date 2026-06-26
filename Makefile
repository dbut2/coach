GO := $(shell go env GOROOT)/bin/go

.PHONY: schemadump
schemadump:
	$(GO) -C tools run ./schemadump

.PHONY: gen-sql
gen-sql:
	$(GO) -C tools tool sqlc generate -f ../db/sqlc.yaml

.PHONY: gen-templ
gen-templ:
	$(GO) -C tools tool templ generate -path ../go/web

.PHONY: screenshots
screenshots: gen-templ
	$(GO) -C tools run ./screenshots

.PHONY: gen-strava
gen-strava:
	rm -rf ./go/clients/strava && mkdir -p ./go/clients/strava
	$(GO) -C tools tool swagger generate client -c strava -m strava/models -e -f https://developers.strava.com/swagger/swagger.json -t ../go/clients --skip-validation
	$(GO) -C go mod tidy

.PHONY: tooling
tooling:
	brew install go
	brew install direnv

.PHONY: gen-mocks
gen-mocks:
	$(GO) -C go tool mockery --config .mockery.yaml
