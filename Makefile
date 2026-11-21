.PHONY: gen lint test bench

gen:
	buf generate

lint:
	buf lint
	go vet ./...

test:
	go test -race ./...

bench:
	go test -run '^$$' -bench . -benchmem ./pkg/...
