.PHONY: build generate ui test clean

build: generate
	go build -o bin/dx ./cmd/dx
	go build -o bin/dx-server ./cmd/dx-server
	go build -o bin/db ./cmd/db

generate:
	go run github.com/mailru/easyjson/easyjson -all internal/apitypes/types.go
	~/go/bin/tygo generate

ui: generate
	cd ui && npm ci && npm run build

test:
	go test ./...

clean:
	rm -rf bin/ ui/dist/
