.PHONY: build ui test clean gen-dxclient

build:
	go build -o bin/dx ./cmd/dx
	go build -o bin/dx-server ./cmd/dx-server
	go build -o bin/db ./cmd/db

gen-dxclient:
	go run ./cmd/dx-client-gen

ui:
	cd ui && npm ci && npm run build

test:
	go test ./...

clean:
	rm -rf bin/ ui/dist/
