BINARY := bin/mcp-diff
IMAGE := mcp-diff:local
VERSION ?= dev
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build check clean docker-build fmt fmt-check test vet

build:
	mkdir -p bin
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(BINARY) .

test:
	go test ./...

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "$$files"; \
		exit 1; \
	fi

vet:
	go vet ./...

check: fmt-check vet test

docker-build:
	docker build --tag $(IMAGE) .

clean:
	rm -rf bin
