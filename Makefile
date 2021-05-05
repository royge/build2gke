# Run tests inside docker container.
test:
	go test -v ./... -failfast -race -count=1

# Run automated linting
lint:
	go mod vendor
	golangci-lint run -v

# Show test coverage in html file.
# `make test` should be executed prior to this command.
coverage:
	go tool cover -html=coverage.out

echo:
	echo "testing only"
