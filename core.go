package main

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

const bo3AppID = "311210"

var digitsOnly = regexp.MustCompile(`^[0-9]{6,20}$`)

func parseWorkshopInputs(input string) ([]string, []error) {
	normalized := strings.NewReplacer("\r", "\n", ",", "\n", ";", "\n", "\t", "\n").Replace(input)
	parts := strings.Split(normalized, "\n")
	seen := map[string]bool{}
	ids := make([]string, 0, len(parts))
	var errs []error

	for _, raw := range parts {
		token := strings.TrimSpace(raw)
		if token == "" {
			continue
		}
		id, err := workshopIDFromToken(token)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", token, err))
			continue
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, errs
}

func workshopIDFromToken(token string) (string, error) {
	token = strings.TrimSpace(token)
	if digitsOnly.MatchString(token) {
		return token, nil
	}
	u, err := url.Parse(token)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", errors.New("not a Workshop URL or numeric item ID")
	}
	host := strings.ToLower(u.Hostname())
	if host != "steamcommunity.com" && !strings.HasSuffix(host, ".steamcommunity.com") {
		return "", errors.New("URL is not on steamcommunity.com")
	}
	id := strings.TrimSpace(u.Query().Get("id"))
	if !digitsOnly.MatchString(id) {
		return "", errors.New("Workshop URL has no valid id parameter")
	}
	return id, nil
}

func classifyItem(tags []string) string {
	hasMap := false
	hasMod := false
	for _, tag := range tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "map", "maps":
			hasMap = true
		case "mod", "mods":
			hasMod = true
		}
	}
	if hasMap {
		return "Maps"
	}
	if hasMod {
		return "Mods"
	}
	return "Other"
}

func sanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			b.WriteRune('_')
			continue
		}
		if unicode.IsSpace(r) {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	out = strings.TrimRight(out, ". ")
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	if out == "" {
		return "Workshop Item"
	}
	upper := strings.ToUpper(out)
	reserved := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true}
	if reserved[upper] || regexp.MustCompile(`^(COM|LPT)[1-9]$`).MatchString(upper) {
		out = "_" + out
	}
	if len(out) > 120 {
		out = strings.TrimSpace(out[:120])
	}
	return out
}

func workshopContentPath(steamcmdRoot, id string) string {
	return filepath.Join(steamcmdRoot, "steamapps", "workshop", "content", bo3AppID, id)
}

func destinationPath(outputRoot, category, title, id string) string {
	safe := sanitizeFolderName(title)
	return filepath.Join(outputRoot, category, fmt.Sprintf("%s [%s]", safe, id))
}
