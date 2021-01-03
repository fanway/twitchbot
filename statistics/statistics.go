package statistics

import (
	"net/http"
	"time"
	"twitchStats/request"
)

type Stats struct {
	MsgCount     int
	MsgCountPrev int
	WatchTime    time.Duration
	LastCheck    time.Time
}

type ChatData struct {
	Links struct {
	} `json:"_links"`
	ChatterCount int `json:"chatter_count"`
	Chatters     struct {
		Broadcaster []string `json:"broadcaster"`
		Vips        []string `json:"vips"`
		Moderators  []string `json:"moderators"`
		Staff       []string `json:"staff"`
		Admins      []string `json:"admins"`
		GlobalMods  []string `json:"global_mods"`
		Viewers     []string `json:"viewers"`
	} `json:"chatters"`
}

func GetUsers(channel string) (map[string]struct{}, error) {
	url := "https://tmi.twitch.tv/group/user/" + channel + "/chatters"
	req, _ := http.NewRequest("GET", url, nil)
	var chatData ChatData
	err := request.JSON(req, 10, &chatData)
	if err != nil {
		return nil, err
	}
	m := make(map[string]struct{})
	for _, name := range chatData.Chatters.Vips {
		m[name] = struct{}{}
	}

	for _, name := range chatData.Chatters.Moderators {
		m[name] = struct{}{}
	}

	for _, name := range chatData.Chatters.Viewers {
		m[name] = struct{}{}
	}

	for _, name := range chatData.Chatters.Broadcaster {
		m[name] = struct{}{}
	}
	return m, nil
}
