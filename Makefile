.PHONY: build ui test clean

build:
	go build -o bin/dx ./cmd/dx
	go build -o bin/dx-server ./cmd/dx-server
	go build -o bin/db ./cmd/db

ui:
	cd ui && npm ci && npm run build

test:
	go test ./...

clean:
	rm -rf bin/ ui/dist/
