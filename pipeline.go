package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const steamCMDZipURL = "https://steamcdn-a.akamaihd.net/client/installer/steamcmd.zip"
const maxDownloadAttempts = 6

type BatchCallbacks struct {
	Log    func(string)
	Status func(string)
}

type itemInfoSidecar struct {
	AppID        string   `json:"app_id"`
	WorkshopID   string   `json:"workshop_id"`
	Title        string   `json:"title"`
	Category     string   `json:"category"`
	FileSize     int64    `json:"file_size"`
	Tags         []string `json:"tags"`
	WorkshopURL  string   `json:"workshop_url"`
	DownloadedAt string   `json:"downloaded_at_utc"`
}

func humanBytes(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	if n < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	}
	return fmt.Sprintf("%.2f GB", float64(n)/(1024*1024*1024))
}

func steamDownloadSucceeded(output, id string) bool {
	low := strings.ToLower(output)
	return strings.Contains(low, "success. downloaded item "+strings.ToLower(id)) ||
		(strings.Contains(low, "success") && strings.Contains(low, "downloaded item "+strings.ToLower(id)))
}

func runBatch(ctx context.Context, ids []string, outputRoot string, cb BatchCallbacks) error {
	logf := cb.Log
	if logf == nil {
		logf = func(string) {}
	}
	statusf := cb.Status
	if statusf == nil {
		statusf = func(string) {}
	}
	if len(ids) == 0 {
		return errors.New("no Workshop items supplied")
	}
	if strings.TrimSpace(outputRoot) == "" {
		return errors.New("output folder is empty")
	}
	if err := os.MkdirAll(outputRoot, 0o755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		cacheRoot = os.TempDir()
	}
	steamRoot := filepath.Join(cacheRoot, "NEXUS_BO3_Workshop_Grabber", "steamcmd")
	statusf("Preparing SteamCMD...")
	steamExe, err := ensureSteamCMD(ctx, steamRoot, logf)
	if err != nil {
		return err
	}

	for i, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		statusf(fmt.Sprintf("Item %d/%d — reading metadata...", i+1, len(ids)))
		meta, metaErr := fetchWorkshopMetadata(id)
		if metaErr != nil {
			logf(fmt.Sprintf("[%s] Metadata warning: %v", id, metaErr))
			meta = WorkshopMetadata{ID: id, Title: "Workshop Item " + id}
		} else {
			size := "unknown size"
			if meta.FileSize > 0 {
				size = humanBytes(meta.FileSize)
			}
			logf(fmt.Sprintf("[%s] %s (%s)", id, meta.Title, size))
		}
		category := classifyItem(meta.Tags)
		dest := destinationPath(outputRoot, category, meta.Title, id)
		if nonEmptyDir(dest) {
			logf(fmt.Sprintf("[%s] Already downloaded — skipped: %s", id, dest))
			continue
		}

		var combined string
		success := false
		for attempt := 1; attempt <= maxDownloadAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			statusf(fmt.Sprintf("Item %d/%d — downloading %s (attempt %d/%d)...", i+1, len(ids), meta.Title, attempt, maxDownloadAttempts))
			logf(fmt.Sprintf("[%s] SteamCMD attempt %d/%d", id, attempt, maxDownloadAttempts))
			args := []string{"+login", "anonymous", "+workshop_download_item", bo3AppID, id}
			if attempt > 1 {
				args = append(args, "validate")
			}
			args = append(args, "+quit")
			out, runErr := runSteamProcess(ctx, steamExe, args, func(line string) {
				lower := strings.ToLower(line)
				if strings.Contains(lower, "downloading item") || strings.Contains(lower, "success") || strings.Contains(lower, "error") || strings.Contains(lower, "update state") {
					logf("  " + strings.TrimSpace(line))
				}
			})
			combined += "\n" + out
			contentDir := workshopContentPath(steamRoot, id)
			if steamDownloadSucceeded(out, id) && nonEmptyDir(contentDir) {
				success = true
				break
			}
			if strings.Contains(strings.ToLower(out), "no subscription") {
				return fmt.Errorf("Steam denied Workshop item %s for anonymous access; Steam account authentication may be required", id)
			}
			if runErr != nil {
				logf(fmt.Sprintf("[%s] SteamCMD returned: %v", id, runErr))
			}
			if attempt < maxDownloadAttempts {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(2 * time.Second):
				}
			}
		}
		if !success {
			return fmt.Errorf("Workshop item %s did not complete after %d attempts; last SteamCMD output: %s", id, maxDownloadAttempts, tailText(combined, 700))
		}
		src := workshopContentPath(steamRoot, id)
		statusf(fmt.Sprintf("Item %d/%d — organizing %s...", i+1, len(ids), meta.Title))
		if err := moveDir(src, dest); err != nil {
			return fmt.Errorf("organize Workshop item %s: %w", id, err)
		}
		if err := writeSidecar(dest, meta, category); err != nil {
			logf(fmt.Sprintf("[%s] Sidecar metadata warning: %v", id, err))
		}
		logf(fmt.Sprintf("[%s] DONE -> %s", id, dest))
	}
	statusf(fmt.Sprintf("Complete — %d item(s) processed.", len(ids)))
	return nil
}

func ensureSteamCMD(ctx context.Context, root string, logf func(string)) (string, error) {
	exe := filepath.Join(root, "steamcmd.exe")
	if fileExists(exe) {
		return exe, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create SteamCMD folder: %w", err)
	}
	logf("SteamCMD not found. Downloading Valve's official SteamCMD package...")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, steamCMDZipURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "NEXUS-BO3-Workshop-Grabber/0.1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download SteamCMD: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("download SteamCMD: HTTP %s", resp.Status)
	}
	zipPath := filepath.Join(root, "steamcmd.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(f, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("save SteamCMD package: %w", copyErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	defer os.Remove(zipPath)
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open SteamCMD package: %w", err)
	}
	defer zr.Close()
	found := false
	for _, zf := range zr.File {
		if strings.EqualFold(filepath.Base(zf.Name), "steamcmd.exe") {
			in, err := zf.Open()
			if err != nil {
				return "", err
			}
			out, err := os.Create(exe)
			if err != nil {
				in.Close()
				return "", err
			}
			_, err = io.Copy(out, in)
			in.Close()
			out.Close()
			if err != nil {
				return "", err
			}
			found = true
			break
		}
	}
	if !found || !fileExists(exe) {
		return "", errors.New("SteamCMD package did not contain steamcmd.exe")
	}
	logf("SteamCMD installed. It will self-update automatically when launched.")
	return exe, nil
}

func writeSidecar(dest string, meta WorkshopMetadata, category string) error {
	info := itemInfoSidecar{
		AppID: bo3AppID, WorkshopID: meta.ID, Title: meta.Title, Category: category,
		FileSize: meta.FileSize, Tags: meta.Tags,
		WorkshopURL: "https://steamcommunity.com/sharedfiles/filedetails/?id=" + meta.ID,
		DownloadedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dest+".nexus.json", data, 0o644)
}

func moveDir(src, dest string) error {
	if !nonEmptyDir(src) {
		return fmt.Errorf("source content folder is missing or empty: %s", src)
	}
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("destination already exists: %s", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyDir(src, dest); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			in.Close()
			return err
		}
		_, cpErr := io.Copy(out, in)
		inCloseErr := in.Close()
		closeErr := out.Close()
		if cpErr != nil {
			return cpErr
		}
		if inCloseErr != nil {
			return inCloseErr
		}
		return closeErr
	})
}

func nonEmptyDir(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

func tailText(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
