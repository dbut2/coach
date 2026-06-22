.PHONY: goget
goget:
	go -C go get ./...
	go -C tools get ./...

.PHONY: schemadump
schemadump:
	go -C tools run ./schemadump

.PHONY: gen-sql
gen-sql:
	go -C tools tool sqlc generate -f ../db/sqlc.yaml

.PHONY: gen-strava
gen-strava:
	rm -rf ./go/client/strava && mkdir -p ./go/client/strava
	go -C tools tool swagger generate client -c strava -m strava/models -e -f https://developers.strava.com/swagger/swagger.json -t ../go/client --skip-validation
	go -C go mod tidy
