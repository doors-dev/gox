GO ?= go
CARGO ?= cargo
RUSTC ?= rustc
RUST_MANIFEST := rust/Cargo.toml
RUST_SYSROOT := $(shell $(RUSTC) --print sysroot)
RUST_OBJCOPY := $(shell $(RUSTC) --print target-libdir)/../bin/rust-objcopy
OBJCOPY_ENV := DYLD_LIBRARY_PATH="$(RUST_SYSROOT)/lib" LD_LIBRARY_PATH="$(RUST_SYSROOT)/lib"
OBJCOPY ?= $(shell for c in "$(RUST_OBJCOPY)" llvm-objcopy objcopy; do $(OBJCOPY_ENV) "$$c" --version >/dev/null 2>&1 && { echo "$$c"; break; }; done)
AR := $(shell command -v ar || command -v llvm-ar)
LTO_SECTIONS := --remove-section=__LLVM,__bitcode --remove-section=__LLVM,__cmdline --remove-section=.llvmbc --remove-section=.llvmcmd

.PHONY: build_rust build install tidy clean

build_rust:
	$(CARGO) build --release --locked --manifest-path=$(RUST_MANIFEST)
	rm -rf rust/lib
	mkdir -p rust/lib/obj
	cp rust/target/release/libgox.a rust/lib/libgox.a
	cd rust/lib/obj && \
		$(AR) t ../libgox.a | tr -d '\r' | grep '\.rcgu\.o$$' > ../objs && \
		xargs -n 64 $(AR) x ../libgox.a < ../objs && \
		while read -r o; do $(OBJCOPY_ENV) "$(OBJCOPY)" $(LTO_SECTIONS) "$$o" || exit 1; done < ../objs && \
		xargs -n 64 $(AR) r ../libgox.a < ../objs && \
		$(AR) s ../libgox.a
	rm -rf rust/lib/obj rust/lib/objs

build: build_rust
	$(GO) build ./cmd/gox

install: build_rust
	$(GO) install ./cmd/gox

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean
	rm -f gox
	rm -rf rust/lib
	$(CARGO) clean --manifest-path=$(RUST_MANIFEST)
