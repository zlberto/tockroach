.DEFAULT_GOAL := check

.PHONY: check fmt bench
check:
	@test -z "$$(gofmt -l *.go)" || { gofmt -l *.go; exit 1; }
	go vet ./...
	go build ./...
	go test -race -count=1 ./...

fmt:
	gofmt -w *.go

bench:
	go test -run '^$$' -bench . -benchmem ./...
