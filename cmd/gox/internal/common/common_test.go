package common

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLockPathAndSocketPathPreferXDG(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	t.Setenv("XDG_CACHE_HOME", "")

	lockPath, err := LockPath("demo")
	if err != nil {
		t.Fatalf("LockPath() error = %v", err)
	}
	if !strings.HasPrefix(lockPath, filepath.Join(xdg, app)) || !strings.HasSuffix(lockPath, "demo.lock") {
		t.Fatalf("LockPath() = %q", lockPath)
	}

	socketPath, err := SocketPath("daemon")
	if err != nil {
		t.Fatalf("SocketPath() error = %v", err)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(socketPath, `\\.\pipe\gox\daemon.sock`) {
			t.Fatalf("SocketPath() = %q", socketPath)
		}
		return
	}
	if !strings.HasPrefix(socketPath, filepath.Join(xdg, app)) || !strings.HasSuffix(socketPath, "daemon.sock") {
		t.Fatalf("SocketPath() = %q", socketPath)
	}
}

func TestLockPathFallsBackToCacheDir(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("XDG_CACHE_HOME", cacheRoot)

	lockPath, err := LockPath("demo")
	if err != nil {
		t.Fatalf("LockPath() error = %v", err)
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir() error = %v", err)
	}
	if !strings.HasPrefix(lockPath, filepath.Join(cacheDir, app)) {
		t.Fatalf("LockPath() = %q, want cache fallback under %q", lockPath, cacheDir)
	}

	socketPath, err := SocketPath("daemon")
	if err != nil {
		t.Fatalf("SocketPath() error = %v", err)
	}
	if runtime.GOOS != "windows" && !strings.HasPrefix(socketPath, filepath.Join(cacheDir, app)) {
		t.Fatalf("SocketPath() = %q, want cache fallback under %q", socketPath, cacheDir)
	}
}

func TestJitter(t *testing.T) {
	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s did not panic", name)
			}
		}()
		fn()
	}

	assertPanics("max<=0", func() { Jitter(0, 0) })
	assertPanics("min<0", func() { Jitter(-time.Millisecond, time.Millisecond) })
	assertPanics("max<min", func() { Jitter(2*time.Millisecond, time.Millisecond) })

	start := time.Now()
	Jitter(5*time.Millisecond, 5*time.Millisecond)
	if elapsed := time.Since(start); elapsed < 4*time.Millisecond {
		t.Fatalf("Jitter() slept %v, want at least ~5ms", elapsed)
	}
}
