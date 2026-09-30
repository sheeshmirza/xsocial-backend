.PHONY: all build run test clean docker-build

APP_NAME = server
BUILD_DIR = bin

all: build

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) .

run:
	go run .

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)

lint:
	go vet ./...
