package statistics

import (
	"net/http"
	"time"
	"twitchStats/request"
	"twitchStats/terminal"
)

type Stats struct {
	MsgCount     int
	MsgCountPrev int
	WatchTime    time.Duration
	LastCheck    time.Time
}

func GetUsers(channel string) (map[string]struct{}, error) {
	url := "https://tmi.twitch.tv/group/user/" + channel + "/chatters"
	req, _ := http.NewRequest("GET", url, nil)
	var chatData terminal.ChatData
	err := request.JSON(req, 10, &chatData)
	if err != nil {
		terminal.Output.Log(err)
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
