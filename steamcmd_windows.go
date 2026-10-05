//go:build windows

package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
)

type lineCapture struct {
	mu      sync.Mutex
	all     bytes.Buffer
	partial string
	onLine  func(string)
}

func (w *lineCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, _ := w.all.Write(p)
	text := w.partial + string(p)
	lines := strings.Split(text, "\n")
	w.partial = lines[len(lines)-1]
	for _, line := range lines[:len(lines)-1] {
		line = strings.TrimRight(line, "\r")
		if w.onLine != nil && strings.TrimSpace(line) != "" {
			w.onLine(line)
		}
	}
	return n, nil
}

func (w *lineCapture) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.onLine != nil && strings.TrimSpace(w.partial) != "" {
		w.onLine(strings.TrimRight(w.partial, "\r"))
		w.partial = ""
	}
	return w.all.String()
}

func runSteamProcess(ctx context.Context, exe string, args []string, onLine func(string)) (string, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = filepath.Dir(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	capture := &lineCapture{onLine: onLine}
	cmd.Stdout = capture
	cmd.Stderr = capture
	err := cmd.Run()
	return capture.String(), err
}
