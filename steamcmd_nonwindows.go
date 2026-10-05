//go:build !windows

package main

import (
	"context"
	"errors"
)

func runSteamProcess(ctx context.Context, exe string, args []string, onLine func(string)) (string, error) {
	return "", errors.New("SteamCMD execution is supported by this build on Windows only")
}
