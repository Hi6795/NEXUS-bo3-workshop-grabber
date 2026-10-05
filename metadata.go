package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const steamPublishedFileDetailsURL = "https://api.steampowered.com/ISteamRemoteStorage/GetPublishedFileDetails/v1/"

type WorkshopMetadata struct {
	ID       string
	Title    string
	FileSize int64
	Tags     []string
}

type steamPublishedFileResponse struct {
	Response struct {
		PublishedFileDetails []struct {
			PublishedFileID string `json:"publishedfileid"`
			Result          int    `json:"result"`
			Title           string `json:"title"`
			FileSize        string `json:"file_size"`
			Tags            []struct {
				Tag string `json:"tag"`
			} `json:"tags"`
		} `json:"publishedfiledetails"`
	} `json:"response"`
}

func fetchWorkshopMetadata(id string) (WorkshopMetadata, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	return fetchWorkshopMetadataWithClient(client, steamPublishedFileDetailsURL, id)
}

func fetchWorkshopMetadataWithClient(client *http.Client, endpoint, id string) (WorkshopMetadata, error) {
	form := url.Values{}
	form.Set("itemcount", "1")
	form.Set("publishedfileids[0]", id)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return WorkshopMetadata{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "NEXUS-BO3-Workshop-Grabber/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return WorkshopMetadata{}, fmt.Errorf("Steam metadata request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return WorkshopMetadata{}, fmt.Errorf("Steam metadata HTTP %s", resp.Status)
	}
	var raw steamPublishedFileResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return WorkshopMetadata{}, fmt.Errorf("invalid Steam metadata response: %w", err)
	}
	if len(raw.Response.PublishedFileDetails) != 1 {
		return WorkshopMetadata{}, fmt.Errorf("Steam returned no metadata for Workshop item %s", id)
	}
	d := raw.Response.PublishedFileDetails[0]
	if d.Result != 1 {
		return WorkshopMetadata{}, fmt.Errorf("Steam returned result %d for Workshop item %s", d.Result, id)
	}
	size, _ := strconv.ParseInt(d.FileSize, 10, 64)
	tags := make([]string, 0, len(d.Tags))
	for _, t := range d.Tags {
		if strings.TrimSpace(t.Tag) != "" {
			tags = append(tags, t.Tag)
		}
	}
	title := strings.TrimSpace(d.Title)
	if title == "" {
		title = "Workshop Item " + id
	}
	return WorkshopMetadata{ID: d.PublishedFileID, Title: title, FileSize: size, Tags: tags}, nil
}
