package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchWorkshopMetadataParsesSteamResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		s := string(body)
		if !strings.Contains(s, "publishedfileids%5B0%5D=1795684736") || !strings.Contains(s, "itemcount=1") {
			t.Fatalf("unexpected form body: %s", s)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"response":{"publishedfiledetails":[{"publishedfileid":"1795684736","result":1,"title":"Worlds Smallest Map","file_size":"150613786","tags":[{"tag":"Map"},{"tag":"Mod"},{"tag":"Zombies"}]}]}}`)
	}))
	defer srv.Close()

	m, err := fetchWorkshopMetadataWithClient(srv.Client(), srv.URL, "1795684736")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.ID != "1795684736" || m.Title != "Worlds Smallest Map" || m.FileSize != 150613786 {
		t.Fatalf("unexpected metadata: %+v", m)
	}
	if got := classifyItem(m.Tags); got != "Maps" {
		t.Fatalf("category got %q", got)
	}
}

func TestFetchWorkshopMetadataRejectsNonSuccessResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"response":{"publishedfiledetails":[{"publishedfileid":"1234567890","result":9}]}}`)
	}))
	defer srv.Close()

	if _, err := fetchWorkshopMetadataWithClient(srv.Client(), srv.URL, "1234567890"); err == nil {
		t.Fatal("expected an error for non-success Steam result")
	}
}
