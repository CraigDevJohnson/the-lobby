.PHONY: test build lambda run fmt vet

test:
	go test ./...

fmt:
	gofmt -l -w cmd internal

vet:
	go vet ./...

# The server for a developer's computer: memory store, sign-in disabled.
run:
	go run ./cmd/site

# The Lambda zip: one static arm64 binary named bootstrap (ADR 0002).
lambda:
	mkdir -p dist/site
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o dist/site/bootstrap ./cmd/site
	cd dist/site && rm -f ../site.zip && zip -q -X ../site.zip bootstrap
	@ls -l dist/site.zip
