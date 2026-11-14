GO ?= go
CARGO ?= cargo
RUSTC ?= rustc
RUST_MANIFEST := rust/Cargo.toml
RUST_LIB := rust/lib
RUST_OBJCOPY := $(shell $(RUSTC) --print target-libdir)/../bin/rust-objcopy
OBJCOPY ?= $(firstword $(wildcard $(RUST_OBJCOPY) $(RUST_OBJCOPY).exe) $(shell command -v llvm-objcopy) objcopy)

.PHONY: build_rust build install tidy clean

build_rust:
	$(CARGO) build --release --locked --manifest-path=$(RUST_MANIFEST)
	mkdir -p $(RUST_LIB)
	cp rust/target/release/libgox.a $(RUST_LIB)/libgox.a
	$(OBJCOPY) --remove-section=__LLVM,__bitcode --remove-section=__LLVM,__cmdline --remove-section=.llvmbc --remove-section=.llvmcmd $(RUST_LIB)/libgox.a

build: build_rust
	$(GO) build ./cmd/gox

install: build_rust
	$(GO) install ./cmd/gox

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean
	rm -f gox $(RUST_LIB)/libgox.a
	$(CARGO) clean --manifest-path=$(RUST_MANIFEST)
