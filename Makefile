.PHONY: goget
goget:
	go -C go get ./...
	go -C tools get ./...

.PHONY: generate
generate:
	go -C go tool sqlc generate -f ../db/sqlc.yaml

.PHONY: schemadump
schemadump:
	go -C tools run ./schemadump
