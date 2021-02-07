package spotify

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"twitchStats/request"
	"twitchStats/terminal"
)

var (
	_, b, _, _ = runtime.Caller(0)
	basepath   = filepath.Dir(b)
	name       = "/sp.json"
	path       = basepath + name
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

type Current struct {
	Context struct {
		ExternalUrls struct {
			Spotify string `json:"spotify"`
		} `json:"external_urls"`
		Href string `json:"href"`
		Type string `json:"type"`
		URI  string `json:"uri"`
	} `json:"context"`
	Timestamp            int64  `json:"timestamp"`
	ProgressMs           int    `json:"progress_ms"`
	IsPlaying            bool   `json:"is_playing"`
	CurrentlyPlayingType string `json:"currently_playing_type"`
	Item                 struct {
		Album struct {
			AlbumType    string `json:"album_type"`
			ExternalUrls struct {
				Spotify string `json:"spotify"`
			} `json:"external_urls"`
			Href   string `json:"href"`
			ID     string `json:"id"`
			Images []struct {
				Height int    `json:"height"`
				URL    string `json:"url"`
				Width  int    `json:"width"`
			} `json:"images"`
			Name string `json:"name"`
			Type string `json:"type"`
			URI  string `json:"uri"`
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
		Href        string `json:"href"`
		ID          string `json:"id"`
		Name        string `json:"name"`
		Popularity  int    `json:"popularity"`
		PreviewURL  string `json:"preview_url"`
		TrackNumber int    `json:"track_number"`
		Type        string `json:"type"`
		URI         string `json:"uri"`
	} `json:"item"`
}

type Playlist struct {
	Collaborative bool   `json:"collaborative"`
	Description   string `json:"description"`
	ExternalUrls  struct {
		Spotify string `json:"spotify"`
	} `json:"external_urls"`
	Followers struct {
		Href  interface{} `json:"href"`
		Total int         `json:"total"`
	} `json:"followers"`
	Href   string `json:"href"`
	ID     string `json:"id"`
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Name  string `json:"name"`
	Owner struct {
		ExternalUrls struct {
			Spotify string `json:"spotify"`
		} `json:"external_urls"`
		Href string `json:"href"`
		ID   string `json:"id"`
		Type string `json:"type"`
		URI  string `json:"uri"`
	} `json:"owner"`
	Public     interface{} `json:"public"`
	SnapshotID string      `json:"snapshot_id"`
	Tracks     struct {
		Href  string `json:"href"`
		Items []struct {
			AddedAt time.Time `json:"added_at"`
			AddedBy struct {
				ExternalUrls struct {
					Spotify string `json:"spotify"`
				} `json:"external_urls"`
				Href string `json:"href"`
				ID   string `json:"id"`
				Type string `json:"type"`
				URI  string `json:"uri"`
			} `json:"added_by"`
			IsLocal bool `json:"is_local"`
			Track   struct {
				Album struct {
					AlbumType        string   `json:"album_type"`
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
					Name string `json:"name"`
					Type string `json:"type"`
					URI  string `json:"uri"`
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
				Href        string `json:"href"`
				ID          string `json:"id"`
				Name        string `json:"name"`
				Popularity  int    `json:"popularity"`
				PreviewURL  string `json:"preview_url"`
				TrackNumber int    `json:"track_number"`
				Type        string `json:"type"`
				URI         string `json:"uri"`
			} `json:"track"`
		} `json:"items"`
		Limit    int         `json:"limit"`
		Next     string      `json:"next"`
		Offset   int         `json:"offset"`
		Previous interface{} `json:"previous"`
		Total    int         `json:"total"`
	} `json:"tracks"`
	Type string `json:"type"`
	URI  string `json:"uri"`
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
	file, err := ioutil.ReadFile(path)
	if err != nil {
		terminal.Output.Log(err)
		return ""
	}
	var data Auth
	err = json.Unmarshal(file, &data)
	if err != nil {
		return ""
	}
	client := base64.StdEncoding.EncodeToString([]byte(data.ClientId + ":" + data.ClientSecret))
	t := time.Since(data.Time)
	if t >= time.Duration(data.Expired)*time.Second {
		body := strings.NewReader(`grant_type=refresh_token&refresh_token=` + data.Refresh)
		url := "https://accounts.spotify.com/api/token"
		req, _ := http.NewRequest("POST", url, body)
		req.Header.Set("Authorization", "Basic "+client)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		var ref Refresh
		err := request.JSON(req, 10, &ref)
		if err != nil {
			return ""
		}
		data.Auth = ref.AccessToken
		data.Expired = ref.ExpiresIn
		data.Time = time.Now()
		w, _ := json.Marshal(data)
		ioutil.WriteFile(path, w, 0644)
	}
	return data.Auth
}

func SearchTrack(name string) (*Search, error) {
	auth := checkAuth()
	name = url.QueryEscape(name)
	url := "https://api.spotify.com/v1/search?query=" + name + "&offset=0&limit=1&type=track"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	var search Search
	err := request.JSON(req, 10, &search)
	if err != nil {
		return nil, err
	}
	return &search, nil
}

func AddToPlaylist(uri string) error {
	auth := checkAuth()
	url := "https://api.spotify.com/v1/playlists/6U9yUDYW4uN845DUERRiMH/tracks?uris=" + uri
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	err := request.JSON(req, 10, nil)
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	return nil
}

func GetCurrentTrack() (string, error) {
	auth := checkAuth()
	url := "https://api.spotify.com/v1/me/player/currently-playing"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	var curr Current
	err := request.JSON(req, 10, &curr)
	if err != nil {
		return "", err
	}
	name := curr.Item.Artists[0].Name
	for i := 1; i < len(curr.Item.Artists); i++ {
		name += ", " + curr.Item.Artists[i].Name
	}
	link := "https://open.spotify.com/track/" + strings.Split(curr.Item.URI, ":")[2]
	return fmt.Sprintf("%s - %s. Link: %s", name, curr.Item.Name, link), nil
}

func SkipToNextTrack() {
	auth := checkAuth()
	url := "https://api.spotify.com/v1/me/player/next"
	req, _ := http.NewRequest("POST", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	err := request.JSON(req, 10, nil)
	if err != nil {
		terminal.Output.Log(err)
		return
	}
}

func RemoveTrack(uri string, pos int) error {
	auth := checkAuth()
	url := "https://api.spotify.com/v1/playlists/6U9yUDYW4uN845DUERRiMH/tracks"
	req, _ := http.NewRequest("DELETE", url, bytes.NewBuffer([]byte("\"tracks\":[{\"uri\":\""+"spotify:track:"+uri+"\", \"positions\": ["+strconv.Itoa(pos)+"]}]")))
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	err := request.JSON(req, 10, nil)
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	return nil
}

func GetTotalInPlaylist() (int, error) {
	auth := checkAuth()
	url := "https://api.spotify.com/v1/playlists/6U9yUDYW4uN845DUERRiMH?fields=tracks.total"
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Content-Type", "application/json")
	var m map[string]interface{}
	err := request.JSON(req, 10, &m)
	if err != nil {
		terminal.Output.Log(err)
		return 0, err
	}
	total, err := m["tracks"].(map[string]interface{})["total"].(json.Number).Int64()
	if err != nil {
		return 0, err
	}
	return int(total), nil
}
