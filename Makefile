.PHONY: bench
bench:
	go test -bench=. -benchmem

run:
	go run main.go
