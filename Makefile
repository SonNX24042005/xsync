.PHONY: all build install test clean run

BINARY_NAME=xsync
BUILD_DIR=bin
INSTALL_DIR=$(HOME)/.local/bin

all: test build

build:
	@mkdir -p $(BUILD_DIR)
	@echo "Dang bien dich $(BINARY_NAME)..."
	go build -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/xsync
	@echo "Bien dich thanh cong: $(BUILD_DIR)/$(BINARY_NAME)"

install: build
	@mkdir -p $(INSTALL_DIR)
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Da cai dat $(BINARY_NAME) vao $(INSTALL_DIR)/$(BINARY_NAME)"

test:
	go test -v ./...

clean:
	@rm -rf $(BUILD_DIR)
	@echo "Da don dep thu muc build."

run:
	go run ./cmd/xsync
