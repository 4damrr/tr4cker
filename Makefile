APP := tr4cker
BIN := bin/$(APP)
ADDR ?= :8080
DB_PATH ?= data/tr4cker.db

.PHONY: tidy build run dev clean

tidy:
	go mod tidy

build:
	go build -o $(BIN) ./cmd/server

run: build
	ADDR=$(ADDR) DB_PATH=$(DB_PATH) ./$(BIN)

dev:
	ADDR=$(ADDR) DB_PATH=$(DB_PATH) go run ./cmd/server

clean:
	rm -rf bin
