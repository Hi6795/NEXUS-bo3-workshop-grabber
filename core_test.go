package main

import (
	"strings"
	"testing"
)

func TestParseWorkshopInputAcceptsURLAndRawID(t *testing.T) {
	input := `https://steamcommunity.com/sharedfiles/filedetails/?id=2801151115
1795684736
https://steamcommunity.com/sharedfiles/filedetails/?foo=bar&id=1234567890&searchtext=x`
	got, errs := parseWorkshopInputs(input)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []string{"2801151115", "1795684736", "1234567890"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] got %q want %q", i, got[i], want[i])
		}
	}
}

func TestParseWorkshopInputDeduplicatesAndRejectsGarbage(t *testing.T) {
	input := "1795684736\n1795684736\nnot-a-workshop-item"
	got, errs := parseWorkshopInputs(input)
	if len(got) != 1 || got[0] != "1795684736" {
		t.Fatalf("unexpected parsed ids: %v", got)
	}
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "not-a-workshop-item") {
		t.Fatalf("expected one descriptive invalid-input error, got: %v", errs)
	}
}

func TestClassifyWorkshopItemPrefersMapTag(t *testing.T) {
	if got := classifyItem([]string{"Mod", "Zombies", "Map"}); got != "Maps" {
		t.Fatalf("got %q want Maps", got)
	}
	if got := classifyItem([]string{"Zombies", "Mod"}); got != "Mods" {
		t.Fatalf("got %q want Mods", got)
	}
	if got := classifyItem([]string{"Weapon"}); got != "Other" {
		t.Fatalf("got %q want Other", got)
	}
}

func TestSanitizeFolderNameRemovesWindowsInvalidCharacters(t *testing.T) {
	got := sanitizeFolderName(`Worlds: Smallest / Map? * "test" <x> |`)
	if strings.ContainsAny(got, `<>:"/\\|?*`) {
		t.Fatalf("still contains invalid windows characters: %q", got)
	}
	if got == "" {
		t.Fatal("sanitized name unexpectedly empty")
	}
}

func TestSteamWorkshopContentPath(t *testing.T) {
	got := workshopContentPath(`C:\\Tools\\steamcmd`, "1795684736")
	normalized := strings.ReplaceAll(got, "\\", "/")
	if !strings.HasSuffix(normalized, "/steamapps/workshop/content/311210/1795684736") {
		t.Fatalf("unexpected content path: %q", got)
	}
}
