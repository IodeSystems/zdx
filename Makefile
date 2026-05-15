.PHONY: build ui test clean gen-dxclient

build:
	go build -o bin/dx ./cmd/dx
	go build -o bin/dx-server ./cmd/dx-server
	go build -o bin/db ./cmd/db
	go build -o bin/dx-agent ./cmd/dx-agent
	go build -o bin/dx-envd ./cmd/dx-envd

gen-dxclient:
	go run ./cmd/dx-client-gen

ui:
	cd ui && pnpm install --frozen-lockfile && pnpm run build

test:
	go test ./...

clean:
	rm -rf bin/ ui/dist/
