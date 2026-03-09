.PHONY: build serve test e2e e2e-cleanup

PORT := 8080

build:
	go build ./cmd/ccasses

serve: build
	./ccasses serve --port $(PORT)

test:
	go test ./...

e2e: build ## サーバーを起動して e2e テストを実行
	go test -tags=e2e -v -count=1 ./e2e/...
