package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	LOW = iota
	MIDDLE
	TOP
)

type SmartVote struct {
	Options map[string]int
	Votes   map[string]string
}

type Utils struct {
	SmartVote SmartVote
}

type Bot struct {
	Channel   string
	Name      string
	Port      string
	OAuth     string
	Server    string
	Conn      net.Conn
	File      *os.File
	Authority map[string]int
	Status    string
	Commands  map[string]*Command
	Utils     Utils
}

type Bttv struct {
	ID            string   `json:"id"`
	Bots          []string `json:"bots"`
	ChannelEmotes []struct {
		ID        string `json:"id"`
		Code      string `json:"code"`
		ImageType string `json:"imageType"`
		UserID    string `json:"userId"`
	} `json:"channelEmotes"`
	SharedEmotes []struct {
		ID        string `json:"id"`
		Code      string `json:"code"`
		ImageType string `json:"imageType"`
		User      struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
			ProviderID  string `json:"providerId"`
		} `json:"user"`
	} `json:"sharedEmotes"`
}

type Ffz struct {
	Room struct {
		_ID            int         `json:"_id"`
		CSS            interface{} `json:"css"`
		DisplayName    string      `json:"display_name"`
		ID             string      `json:"id"`
		IsGroup        bool        `json:"is_group"`
		ModUrls        interface{} `json:"mod_urls"`
		ModeratorBadge interface{} `json:"moderator_badge"`
		Set            int         `json:"set"`
		TwitchID       int         `json:"twitch_id"`
		UserBadges     struct {
		} `json:"user_badges"`
	} `json:"room"`
	Sets struct {
		IDX struct {
			Type        int         `json:"_type"`
			CSS         interface{} `json:"css"`
			Description interface{} `json:"description"`
			Emoticons   []struct {
				CSS      interface{} `json:"css"`
				Height   int         `json:"height"`
				Hidden   bool        `json:"hidden"`
				ID       int         `json:"id"`
				Margins  interface{} `json:"margins"`
				Modifier bool        `json:"modifier"`
				Name     string      `json:"name"`
				Offset   interface{} `json:"offset"`
				Owner    struct {
					ID          int    `json:"_id"`
					DisplayName string `json:"display_name"`
					Name        string `json:"name"`
				} `json:"owner"`
				Public bool `json:"public"`
				Urls   struct {
					One  string `json:"1,omitempty"`
					Two  string `json:"2,omitempty"`
					Four string `json:"4,omitempty"`
				} `json:"urls,omitempty"`
				Width int `json:"width"`
			} `json:"emoticons"`
			Icon  interface{} `json:"icon"`
			ID    int         `json:"id"`
			Title string      `json:"title"`
		} `json:`
	} `json:"sets"`
}

type TwitchEmotes []struct {
	ID             int    `json:"id"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	State          string `json:"state"`
	Regex          string `json:"regex"`
	EmoticonSet    int    `json:"emoticon_set"`
	URL            string `json:"url"`
	SubscriberOnly bool   `json:"subscriber_only"`
}

type bttvGlobal []struct {
	ID        string `json:"id"`
	Code      string `json:"code"`
	ImageType string `json:"imageType"`
	UserID    string `json:"userId"`
}

// connects to twitch chat
func (bot *Bot) Connect(wg *sync.WaitGroup) {
	var err error
	bot.Conn, err = net.Dial("tcp", bot.Server+":"+bot.Port)
	if err != nil {
		fmt.Printf("Unable to connect!")
	}
	fmt.Fprintf(bot.Conn, "CAP REQ :twitch.tv/tags\r\n")
	fmt.Fprintf(bot.Conn, "PASS %s\r\n", bot.OAuth)
	fmt.Fprintf(bot.Conn, "NICK %s\r\n", bot.Name)
	fmt.Fprintf(bot.Conn, "JOIN %s\r\n", bot.Channel)
	fmt.Printf("connected to %s\n", bot.Channel)
	wg.Add(1)
	bot.initCommands()
	go bot.reader(wg)
}

func (bot *Bot) Pong(line string) {
	pong := strings.Split(line, "PING")
	fmt.Fprintf(bot.Conn, "PONG %s\r\n", pong[1])
}

func (bot *Bot) SendMessage(msg string) {
	fmt.Fprintf(bot.Conn, "PRIVMSG %s :%s\r\n", bot.Channel, msg)
}

// reader and parser
func (bot *Bot) reader(wg *sync.WaitGroup) {
	tp := textproto.NewReader(bufio.NewReader(bot.Conn))
	w := bufio.NewWriter(bot.File)
	logChan := make(chan Message)
	go logsWriter(w, logChan, wg)
	for {
		line, err := tp.ReadLine()
		if err != nil {
			fmt.Println(err)
			break
		}
		// parsing chat
		go bot.parseChat(line, w, logChan)
		defer wg.Done()
	}
}

type Message struct {
	Username string
	Text     string
}

func logsWriter(w *bufio.Writer, logChan <-chan Message, wg *sync.WaitGroup) {
	for {
		select {
		case message := <-logChan:
			fmt.Fprintf(w, "[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"), message.Username, message.Text)
			w.Flush()
		}
		defer wg.Done()
	}
}

func (bot *Bot) parseChat(line string, w *bufio.Writer, logChan chan<- Message) {
	if strings.Contains(line, "PRIVMSG") {
		line := line[1:]
		userdata := strings.Split(line, ".tmi.twitch.tv PRIVMSG "+bot.Channel)
		username := strings.Split(userdata[0], "@")[1]
		emotes := strings.Split(strings.Split(userdata[0], "emotes=")[1], ";")[0]
		usermessage := strings.Replace(userdata[1], " :", "", 1)
		logChan <- Message{username, usermessage}
		messageLength := len(usermessage)
		if bot.Status == "smartvote" && messageLength == 1 {
			// check if there is that vote option
			if _, ok := bot.Utils.SmartVote.Options[usermessage]; ok {
				// consider only the first vote
				if _, ok := bot.Utils.SmartVote.Votes[username]; !ok {
					bot.Utils.SmartVote.Options[usermessage]++
					bot.Utils.SmartVote.Votes[username] = usermessage
				}
			}
		}
		if messageLength >= 300 && messageLength <= 2000 {
			pasteFile, err := os.OpenFile("paste.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				fmt.Println(err)
			}
			defer pasteFile.Close()
			pasteWriter := bufio.NewWriter(pasteFile)
			fmt.Fprintf(pasteWriter, "%s\n\n", usermessage)
		}

		if usermessage[0] == '!' {
			go bot.processCommands(usermessage, username, emotes)
		}
	} else if strings.HasPrefix(line, "PING") { // response to keep connection alive
		bot.Pong(line)
	}
}

// chat commands
func (bot *Bot) processCommands(message, username, emotes string) {
	var cmd *Command
	var err error
	level := bot.Authority[username]
	cmd, err = bot.ParseCommand(message, emotes, username, level)
	if err != nil {
		bot.Status = "Running"
		fmt.Println(err)
		return
	}
	err = cmd.ExecCommand(level)
	if err != nil {
		bot.Status = "Running"
		fmt.Println(err)
		return
	}
}

func authorityInit() map[string]int {
	file, err := ioutil.ReadFile("authority.txt")
	if err != nil {
		fmt.Println(err)
	}
	var m map[string]string
	if err = json.Unmarshal(file, &m); err != nil {
		fmt.Println(err)
	}
	mp := make(map[string]int)
	for k := range m {
		switch m[k] {
		case "top":
			mp[k] = TOP
		case "middle":
			mp[k] = MIDDLE
		case "low":
			mp[k] = LOW
		}
	}
	return mp
}

func (bot *Bot) ChangeAuthority(username, level string) {
	file, err := ioutil.ReadFile("authority.txt")
	if err != nil {
		fmt.Println(err)
	}
	var m map[string]string
	if err = json.Unmarshal(file, &m); err != nil {
		fmt.Println(err)
	}
	m[username] = level
	newAuthority, err := json.Marshal(m)
	if err != nil {
		fmt.Println(err)
		return
	}
	err = ioutil.WriteFile("authority.txt", newAuthority, 0644)
	if err != nil {
		fmt.Println(err)
		return
	}
	bot.Authority = authorityInit()
}

func (bot *Bot) updateEmotes() {
	db := ConnectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS ffzbttv(url TEXT NOT NULL, code TEXT NOT NULL, UNIQUE (url) ON CONFLICT REPLACE);")
	if err != nil {
		fmt.Println(err)
	}
	ffzUrl := "https://api.frankerfacez.com/v1/room/" + bot.Channel[1:]
	var ffz map[string]interface{}
	req, _ := http.NewRequest("GET", ffzUrl, nil)
	err = RequestJSON(req, 10, &ffz)
	if err != nil {
		fmt.Println(err)
		return
	}
	room := ffz["room"].(map[string]interface{})
	twitchID, _ := room["twitch_id"].(json.Number).Int64()
	set := room["set"].(json.Number).String()
	sets := ffz["sets"].(map[string]interface{})[set].(map[string]interface{})["emoticons"].([]interface{})

	var s string
	for i := range sets {
		urls := sets[i].(map[string]interface{})["urls"].(map[string]interface{})
		name := sets[i].(map[string]interface{})["name"].(string)
		if val, ok := urls["4"]; ok {
			s = val.(string)
		} else if val, ok = urls["2"]; ok {
			s = val.(string)
		} else if val, ok = urls["1"]; ok {
			s = val.(string)
		}
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", "https:"+s, name)
		if err != nil {
			fmt.Println(err)
		}
	}

	bttvUrl := "https://api.betterttv.net/3/cached/users/twitch/" + strconv.FormatInt(twitchID, 10)
	var bttv Bttv
	req, _ = http.NewRequest("GET", bttvUrl, nil)
	err = RequestJSON(req, 10, &bttv)
	if err != nil {
		fmt.Println(err)
	}
	cdnUrl := "https://cdn.betterttv.net/emote/"
	for _, u := range bttv.SharedEmotes {
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", cdnUrl+u.ID+"/3x", u.Code)
		if err != nil {
			fmt.Println(err)
		}
	}
	for _, u := range bttv.ChannelEmotes {
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", cdnUrl+u.ID+"/3x", u.Code)
		if err != nil {
			fmt.Println(err)
		}
	}

	twitchUrl := "https://api.twitch.tv/api/channels/" + bot.Channel[1:] + "/product"
	req, _ = http.NewRequest("GET", twitchUrl, nil)
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID_"))
	var tempTwitch map[string]json.RawMessage
	err = RequestJSON(req, 10, &tempTwitch)
	if err != nil {
		fmt.Println(err)
		return
	}
	var twitch TwitchEmotes
	if err := json.Unmarshal(tempTwitch["emoticons"], &twitch); err != nil {
		fmt.Println(err)
		return
	}

	for _, v := range twitch {
		url := strings.Replace(v.URL, "/1.0", "/3.0", 1)
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", url, v.Regex)
		if err != nil {
			fmt.Println(err)
		}
	}

	tx.Commit()
}

func StartBot(channel string, botInstances map[string]*Bot) {
	wg := new(sync.WaitGroup)
	logfile, err := os.OpenFile(channel+".log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()
	m := authorityInit()
	bot := Bot{
		Channel:   channel,
		Name:      "funwayz",
		Port:      "6667",
		OAuth:     os.Getenv("TWITCH_OAUTH_ENV"),
		Server:    "irc.twitch.tv",
		File:      logfile,
		Conn:      nil,
		Authority: m,
	}
	botInstances[channel] = &bot
	bot.Connect(wg)
	wg.Wait()
}
