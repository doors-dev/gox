GO ?= go
CARGO ?= cargo
RUST_MANIFEST := rust/Cargo.toml

.PHONY: build_rust build install tidy clean

build_rust:
	$(CARGO) build --release --manifest-path=$(RUST_MANIFEST)

build: build_rust
	$(GO) build ./cmd/gox

install: build_rust
	$(GO) install ./cmd/gox

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean
	rm -f gox
	$(CARGO) clean --manifest-path=$(RUST_MANIFEST)
