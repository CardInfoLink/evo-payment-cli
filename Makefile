BINARY   := evo-cli
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
DATE     := $(shell date +%Y-%m-%d)
LDFLAGS  := -s -w -X github.com/evopayment/evo-cli/internal/build.Version=$(VERSION) -X github.com/evopayment/evo-cli/internal/build.Date=$(DATE)
PREFIX   ?= /usr/local

.PHONY: build gen_meta test vet install clean e2e e2e-live-verbose test-all

build: gen_meta
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) .

gen_meta:
	python3 scripts/gen_meta.py \
		--merchant-input evopayment-skills/swagger-merchant-api.json \
		--linkpay-input evopayment-skills/swagger-linkpay-api.json \
		--output internal/registry/meta_data.json

test: vet
	go test -race -count=1 ./...

vet:
	go vet ./...

install: build
	install -m755 $(BINARY) $(PREFIX)/bin/$(BINARY)

clean:
	rm -f $(BINARY)

e2e: build
	@echo "Running E2E tests (offline, no API calls)..."
	bash scripts/e2e_test.sh ./$(BINARY)

e2e-live: build
	@echo "Running Live E2E tests (calls real Evo Payment UAT APIs)..."
	bash scripts/e2e_live_test.sh ./$(BINARY)

e2e-live-verbose: build
	@echo "Running Live E2E tests (verbose — calls real Evo Payment UAT APIs)..."
	bash scripts/e2e_live_test.sh ./$(BINARY) --verbose

test-all: test e2e
	@echo "All tests passed (unit + e2e)"
