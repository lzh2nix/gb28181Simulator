all:
	go build -o gb28181Simulator main.go
	gofmt -w -s .
test:
	go test -cover ./internal/...
