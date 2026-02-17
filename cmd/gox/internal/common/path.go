package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Unix priority:
//  1) $XDG_RUNTIME_DIR/<app>/<sockName>          (best for sockets)
//  2) os.UserCacheDir()/ <app>/run/<sockName>   (user-writable fallback)
//  3) os.TempDir()/ <app>-<uid>/<sockName>      (last resort; may be cleaned)
//
// Windows: returns a Named Pipe path instead of a filesystem path.

const app = "gox"

func LockPath(lockName string) (string, error) {
	lockName += ".lock"
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dir := filepath.Join(xdg, app)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			return filepath.Join(dir, lockName), nil
		}
	}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		dir := filepath.Join(cacheDir, app)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			return filepath.Join(dir, lockName), nil
		}
	}
	dir := filepath.Join(os.TempDir(), app)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create per-user runtime dir: %w", err)
	}
	return filepath.Join(dir, lockName), nil
}

func SocketPath(sockName string) (string, error) {
	sockName += ".sock"
	if runtime.GOOS == "windows" {
		return `\\.\pipe\` + app + `\` + sockName, nil
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dir := filepath.Join(xdg, app)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			p := filepath.Join(dir, sockName)
			return p, nil
		}
	}
	if cacheDir, err := os.UserCacheDir(); err == nil && cacheDir != "" {
		dir := filepath.Join(cacheDir, app)
		if err := os.MkdirAll(dir, 0o700); err == nil {
			p := filepath.Join(dir, sockName)
			return p, nil
		}
	}
	dir := filepath.Join(os.TempDir(), app)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create per-user runtime dir: %w", err)
	}
	p := filepath.Join(dir, sockName)
	return p, nil
}
