# Builds from public sources have no key: the final message stays sealed and
# state is not signed. Official binaries are built with `make release`.

.PHONY: build release seal test

build: ## development build, final message stays sealed
	go build -o lastdose .

seal: ## encrypt assets/final_message.txt into assets/final_message.enc
	go run ./cmd/sealmsg

release: ## build with the key from .final_message.key compiled in
	go run ./cmd/sealmsg -key-only
	go build -tags release -trimpath -o lastdose .

test:
	go vet ./...
	go test ./...
	go test -tags release ./assets
