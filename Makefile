.PHONY: dev dev-api dev-frontend build build-frontend build-go test test-coverage lint lint-go lint-frontend mocks clean test-go test-frontend test-live

dev:
	$(MAKE) -j2 dev-api dev-frontend

dev-api:
	go run .

dev-frontend:
	cd web/frontend && npm run dev

build: build-frontend build-go

build-frontend:
	cd web/frontend && npm ci && npm run build

build-go:
	CGO_ENABLED=0 go build -o bin/voiceline-diary .

test: test-go test-frontend

test-go:
	go test -cover ./...

test-frontend:
	cd web/frontend && npm test

test-live:
	GEMINI_LIVE_TEST=1 go test ./internal/gemini -run TestLive -v

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

lint: lint-go lint-frontend

lint-go:
	golangci-lint run ./...

lint-frontend:
	cd web/frontend && npx --yes prettier --check src

mocks:
	go tool mockery

clean:
	rm -rf bin coverage.out web/frontend/dist
