//go:build demo

package e2e

// Browser demo tests record Playwright video to .zdx/demo/video/.
//
// Requires playwright-go and installed browsers:
//   go get github.com/playwright-community/playwright-go
//   go run github.com/playwright-community/playwright-go/cmd/playwright install --with-deps chromium
//
// Build:  go test -c -tags=demo -o bin/zdx-demo ./test/e2e/
// Run:    ./bin/zdx-demo -test.run TestDemo -test.v
