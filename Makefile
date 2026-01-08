GO ?= go
CARGO ?= cargo
RUST_MANIFEST := rust/Cargo.toml

.PHONY: build_rust build install clean

build_rust:
	$(CARGO) build --release --manifest-path=$(RUST_MANIFEST)

build: build_rust
	$(GO) mod tidy
	$(GO) build ./cmd/gox

install: build_rust
	$(GO) mod tidy
	$(GO) install ./cmd/gox

clean:
	$(GO) clean -cache 
	$(CARGO) clean --manifest-path=$(RUST_MANIFEST)

