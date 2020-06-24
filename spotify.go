package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

type Search struct {
	Tracks struct {
		Href  string `json:"href"`
		Items []struct {
			Album struct {
				AlbumType string `json:"album_type"`
				Artists   []struct {
					ExternalUrls struct {
						Spotify string `json:"spotify"`
					} `json:"external_urls"`
					Href string `json:"href"`
					ID   string `json:"id"`
					Name string `json:"name"`
					Type string `json:"type"`
					URI  string `json:"uri"`
				} `json:"artists"`
				AvailableMarkets []string `json:"available_markets"`
				ExternalUrls     struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href   string `json:"href"`
				ID     string `json:"id"`
				Images []struct {
					Height int    `json:"height"`
					URL    string `json:"url"`
					Width  int    `json:"width"`
				} `json:"images"`
				Name                 string `json:"name"`
				ReleaseDate          string `json:"release_date"`
				ReleaseDatePrecision string `json:"release_date_precision"`
				TotalTracks          int    `json:"total_tracks"`
				Type                 string `json:"type"`
				URI                  string `json:"uri"`
			} `json:"album"`
			Artists []struct {
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href string `json:"href"`
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
				URI  string `json:"uri"`
			} `json:"artists"`
			AvailableMarkets []string `json:"available_markets"`
			DiscNumber       int      `json:"disc_number"`
			DurationMs       int      `json:"duration_ms"`
			Explicit         bool     `json:"explicit"`
			ExternalIds      struct {
				Isrc string `json:"isrc"`
			} `json:"external_ids"`
			ExternalUrls struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Href        string      `json:"href"`
			ID          string      `json:"id"`
			IsLocal     bool        `json:"is_local"`
			Name        string      `json:"name"`
			Popularity  int         `json:"popularity"`
			PreviewURL  interface{} `json:"preview_url"`
			TrackNumber int         `json:"track_number"`
			Type        string      `json:"type"`
			URI         string      `json:"uri"`
		} `json:"items"`
		Limit    int         `json:"limit"`
		Next     string      `json:"next"`
		Offset   int         `json:"offset"`
		Previous interface{} `json:"previous"`
		Total    int         `json:"total"`
	} `json:"tracks"`
}

type Auth struct {
	Auth         string    `json:"auth"`
	Refresh      string    `json:"refresh"`
	Time         time.Time `json:"time"`
	Expired      int       `json:"expired"`
	ClientId     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
}

type Refresh struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	ExpiresIn   int    `json:"expires_in"`
}

func checkAuth() string {
	file, _ := ioutil.ReadFile("sp.json")
	var data Auth
	err := json.Unmarshal(file, &data)
	if err != nil {
		log.Println(err)
		return ""
	}
	fmt.Println(data)
	client := base64.StdEncoding.EncodeToString([]byte(data.ClientId + ":" + data.ClientSecret))
	t := time.Since(data.Time)
	if t >= time.Duration(data.Expired)*time.Second {
		body := strings.NewReader(`grant_type=refresh_token&refresh_token=` + data.Refresh)
		url := "https://accounts.spotify.com/api/token"
		req, _ := http.NewRequest("POST", url, body)
		req.Header.Set("Authorization", "Basic "+client)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var ref Refresh
		err := requestJSON(req, 10, &ref)
		if err != nil {
			log.Println(err)
			return ""
		}
		data.Auth = ref.AccessToken
		data.Expired = ref.ExpiresIn
		data.Time = time.Now()
		w, _ := json.Marshal(data)
		ioutil.WriteFile("sp.json", w, 0644)
	}
	return data.Auth
}

func searchTrack(name string) string {
	auth := checkAuth()
	name = strings.Replace(name, " ", "%20", -1)
	fmt.Println(name)
	url := "https://api.spotify.com/v1/search?query=" + name + "&offset=0&limit=1&type=track"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	var search Search
	err := requestJSON(req, 10, &search)
	if err != nil {
		log.Println(err)
		return ""
	}
	return search.Tracks.Items[0].URI
}

func addToPlaylist(uri string) {
	auth := checkAuth()
	url := "https://api.spotify.com/v1/playlists/6U9yUDYW4uN845DUERRiMH/tracks?uris=" + uri
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	err := requestJSON(req, 10, nil)
	if err != nil {
		fmt.Println(err)
	}
}
