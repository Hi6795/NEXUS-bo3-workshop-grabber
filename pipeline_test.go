package main

import "testing"

func TestSteamOutputSuccessDetection(t *testing.T) {
	good := "Connecting anonymously to Steam Public...OK\nSuccess. Downloaded item 1795684736 to C:\\steamcmd\\steamapps\\workshop\\content\\311210\\1795684736 (123 bytes)\n"
	if !steamDownloadSucceeded(good, "1795684736") {
		t.Fatal("expected success output to be detected")
	}
	bad := "Downloading item 1795684736 ...\nERROR! Download item 1795684736 failed (Timeout).\n"
	if steamDownloadSucceeded(bad, "1795684736") {
		t.Fatal("timeout must not be treated as success")
	}
}

func TestHumanBytes(t *testing.T) {
	if got := humanBytes(143637000); got != "137.0 MB" {
		t.Fatalf("got %q", got)
	}
	if got := humanBytes(1622000000); got != "1.51 GB" {
		t.Fatalf("got %q", got)
	}
}
