.PHONY: build serve test e2e e2e-cleanup

PORT := 8080

build:
	go build ./cmd/ccasses

serve: build
	./ccasses serve --port $(PORT)

test:
	go test ./...

e2e: build ## サーバーを起動して e2e テストを実行
	@echo "Starting server on port $(PORT)..."
	@./ccasses serve --port $(PORT) & echo $$! > .server.pid
	@sleep 2
	@echo "Running e2e tests..."
	@CLAUDECODE= claude -p "e2e_tests.md を読んで、各テスト項目（## 単位）ごとに別々の subagent を起動し、並列で実行してください。各 subagent には agent-browser スキルを使ってテストを実行するよう指示してください。全 subagent の結果を集約して pass/fail を報告してください。" \
		--model claude-sonnet-4-6 \
		--allowedTools "Skill,Read,Agent,Bash,Glob,Grep,ToolSearch" \
		|| true
	@$(MAKE) e2e-cleanup

e2e-cleanup:
	@if [ -f .server.pid ]; then \
		kill $$(cat .server.pid) 2>/dev/null || true; \
		rm -f .server.pid; \
		echo "Server stopped."; \
	fi
