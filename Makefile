.PHONY: build generate sqlgen ui test clean

build: generate
	go build -o bin/dx ./cmd/dx
	go build -o bin/dx-server ./cmd/dx-server

# Generate easyjson marshal code + TypeScript types from internal/apitypes
generate:
	go run github.com/mailru/easyjson/easyjson -all internal/apitypes/types.go
	~/go/bin/tygo generate

# Regenerate typed DB query wrappers from queries/ + migrations/
sqlgen:
	sqlc generate

ui: generate
	cd ui && npm ci && npm run build

test:
	go test ./...

clean:
	rm -rf bin/ ui/dist/
