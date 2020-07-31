package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	//"io/ioutil"
)

type IdData struct {
	Data []struct {
		ID              string `json:"id"`
		Login           string `json:"login"`
		DisplayName     string `json:"display_name"`
		Type            string `json:"type"`
		BroadcasterType string `json:"broadcaster_type"`
		Description     string `json:"description"`
		ProfileImageURL string `json:"profile_image_url"`
		OfflineImageURL string `json:"offline_image_url"`
		ViewCount       int    `json:"view_count"`
	} `json:"data"`
}

type Followers struct {
	Total int `json:"total"`
	Data  []struct {
		FromID     string `json:"from_id"`
		FromName   string `json:"from_name"`
		ToID       string `json:"to_id"`
		ToName     string `json:"to_name"`
		FollowedAt string `json:"followed_at"`
	} `json:"data"`
	Pagination struct {
		Cursor string `json:"cursor"`
	} `json:"pagination"`
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

func requestJSON(req *http.Request, timeout int, obj interface{}) error {
	client := &http.Client{Timeout: time.Second * time.Duration(timeout)}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode > http.StatusIMUsed {
		return errors.New("HTTP status:" + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	decoder.UseNumber()
	err = decoder.Decode(&obj)
	if err != nil {
		return err
	}
	return nil
}

func connectDb() *sql.DB {
	db, err := sql.Open("sqlite3", "./data.db?_busy_timeout=5000&cache=shared&mode=rwc")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	return db
}

func findPerson(name string) {
	db := connectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return
	}
	defer tx.Rollback()
	var id string
	row := tx.QueryRow("SELECT FromId FROM Followers WHERE FromName=$1;", name)
	err = row.Scan(&id)
	u := "https://api.twitch.tv/helix/users?login=" + name
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", "Bearer "+strings.Split(os.Getenv("TWITCH_OAUTH_ENV"), ":")[1])
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))
	if err != nil {
		var iddata IdData
		err := requestJSON(req, 10, &iddata)
		if err != nil {
			log.Println(err)
			return
		}
		id = iddata.Data[0].ID
	}
	req.URL, _ = url.Parse("https://api.twitch.tv/helix/users/follows?first=100&from_id=" + id)
	var followers Followers
	err = requestJSON(req, 10, &followers)
	if err != nil {
		log.Println(err)
		return
	}
	cursor := followers.Pagination.Cursor
	for i := 0; i < followers.Total/100; i++ {
		req.URL, _ = url.Parse("https://api.twitch.tv/helix/users/follows?first=100&from_id=" + id + "&after=" + cursor)
		var temp Followers
		err = requestJSON(req, 10, &temp)
		if err != nil {
			log.Println(err)
			return
		}
		followers.Data = append(followers.Data, temp.Data...)
		cursor = temp.Pagination.Cursor
	}

	_, err = tx.Exec("CREATE TEMPORARY TABLE Follow(Id INTEGER PRIMARY KEY, FromId TEXT, FromName TEXT, ToId TEXT, ToName TEXT, FollowedAt TEXT);")

	if err != nil {
		log.Println(err)
	}

	tx.Exec("UPDATE Followers SET FromName=$1 WHERE FromId=$2", followers.Data[0].FromName, id)
	for _, d := range followers.Data {
		_, err := tx.Exec("INSERT INTO temp.Follow(FromId, FromName, ToId, ToName, FollowedAt) values($1, $2, $3, $4, $5);", d.FromID, d.FromName, d.ToID, d.ToName, d.FollowedAt)
		if err != nil {
			log.Println(err)
		}
	}

	rows, err := tx.Query("SELECT * FROM temp.Follow tm WHERE NOT EXISTS (SELECT * FROM Followers t WHERE tm.FromId = t.FromId AND tm.ToId = t.Toid);")

	defer rows.Close()
	u = "https://tmi.twitch.tv/group/user/" + strings.ToLower(name) + "/chatters"
	req, _ = http.NewRequest("GET", u, nil)
	var chatData ChatData
	err = requestJSON(req, 10, &chatData)
	if err != nil {
		log.Println(err)
	}
	online := false

	if len(chatData.Chatters.Broadcaster) > 0 {
		online = true
	}
	if online {
		fmt.Println(name + " [ONLINE]")
	} else {
		fmt.Println(name + " [OFFLINE]")
	}
	fmt.Println("-----------------------")

	if err != nil {
		log.Println(err)
	}
	fmt.Println("New channels: ")
	for rows.Next() {
		var idx int
		var fromId string
		var fromName string
		var toId string
		var toName string
		var followersAt string
		err = rows.Scan(&idx, &fromId, &fromName, &toId, &toName, &followersAt)
		if err != nil {
			log.Println(err)
		}
		fmt.Print(toName + " ")
		_, err = tx.Exec("INSERT INTO Followers(FromId, FromName, ToId, ToName, FollowedAt) values($1, $2, $3, $4, $5);", fromId, fromName, toId, toName, followersAt)
		if err != nil {
			log.Println(err)
		}
	}
	fmt.Println("")

	rows, err = tx.Query("SELECT t.Id, t.ToName FROM Followers t WHERE t.FromId=$1 AND NOT EXISTS (SELECT * FROM temp.Follow tm WHERE tm.ToId=t.ToId);", id)

	defer rows.Close()
	if err != nil {
		log.Println(err)
	}

	fmt.Println("Unfollowed channels: ")
	for rows.Next() {
		var idx int
		var toName string
		err := rows.Scan(&idx, &toName)
		if err != nil {
			log.Println(err)
		}
		fmt.Print(toName + " ")
		_, err = tx.Exec("DELETE FROM Followers WHERE id=$1", strconv.Itoa(idx))
		if err != nil {
			log.Println(err)
		}
	}
	fmt.Println("")

	rows, err = tx.Query("SELECT ToName FROM Followers WHERE FromId=$1;", id)
	defer rows.Close()
	if err != nil {
		log.Println(err)
	}
	if !online {
		fmt.Println("-----------------------")
		tx.Commit()
		return
	}
	name = strings.ToLower(name)
	fmt.Println("Currently watching: ")
	for rows.Next() {
		var toName string
		rows.Scan(&toName)
		u = "https://tmi.twitch.tv/group/user/" + strings.ToLower(toName) + "/chatters"
		req, _ := http.NewRequest("GET", u, nil)
		var chatData ChatData
		err = requestJSON(req, 10, &chatData)
		if err != nil {
			continue
		}

		for _, cname := range chatData.Chatters.Vips {
			if cname == name {
				fmt.Print(toName + " ")
			}
		}

		for _, cname := range chatData.Chatters.Moderators {
			if cname == name {
				fmt.Print(toName + " ")
			}
		}

		for _, cname := range chatData.Chatters.Viewers {
			if cname == name {
				fmt.Print(toName + " ")
			}
		}
	}
	tx.Commit()
	fmt.Println("\n-----------------------")
}

func personsList(prefix string) []string {
	db := connectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT DISTINCT FromName FROM Followers WHERE FromName LIKE $1;", prefix)
	defer rows.Close()
	if err != nil {
		log.Println(err)
	}
	var buffer []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		buffer = append(buffer, s)
	}
	tx.Commit()
	return buffer
}

type VodsChat struct {
	Comments []Comments `json:"comments"`
	Next     string     `json:"_next,omitempty"`
}

func (chat *VodsChat) parse() []string {
	var messages []string
	for _, comment := range chat.Comments {
		username := comment.Commenter.DisplayName
		currentTime := time.Now()
		timeCreated := comment.CreatedAt.In(currentTime.Location()).Format("2006-01-02 15:04:05 -0700 MST")
		timeOffset := time.Date(0, 0, 0, 0, 0, int(comment.ContentOffsetSeconds), 0, time.UTC).Format("15:04:05")
		msg := comment.Message.Body
		messages = append(messages, fmt.Sprintf("[%s] %s: %s [%s]\n", timeCreated, username, msg, timeOffset))
	}
	return messages
}

type Commenter struct {
	DisplayName string      `json:"display_name"`
	ID          string      `json:"_id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Bio         interface{} `json:"bio"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Logo        string      `json:"logo"`
}

type UserBadges struct {
	ID      string `json:"_id"`
	Version string `json:"version"`
}

type VodMessage struct {
	Body             string       `json:"body"`
	Emoticons        []Emoticons  `json:"emoticons,omitempty"`
	Fragments        []Fragments  `json:"fragments"`
	IsAction         bool         `json:"is_action"`
	UserBadges       []UserBadges `json:"user_badges,omitempty"`
	UserColor        string       `json:"user_color,omitempty"`
	UserNoticeParams interface{}  `json:"user_notice_params,omitempty"`
}

type Fragments struct {
	Text     string   `json:"text"`
	Emoticon Emoticon `json:"emoticon,omitempty"`
}

type Emoticon struct {
	EmoticonID    string `json:"emoticon_id"`
	EmoticonSetID string `json:"emoticon_set_id"`
}

type Emoticons struct {
	ID    string `json:"_id"`
	Begin int    `json:"begin"`
	End   int    `json:"end"`
}

type Comments struct {
	ID                   string     `json:"_id"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	ChannelID            string     `json:"channel_id"`
	ContentType          string     `json:"content_type"`
	ContentID            string     `json:"content_id"`
	ContentOffsetSeconds float64    `json:"content_offset_seconds"`
	Commenter            Commenter  `json:"commenter"`
	Source               string     `json:"source"`
	State                string     `json:"state"`
	Message              VodMessage `json:"message,omitempty"`
}

func extractId(url string) string {
	re := regexp.MustCompile(`https://www\.twitch\.tv/videos/(.*)`)
	s := re.FindStringSubmatch(url)
	if s != nil {
		return s[1]
	}
	return url
}

func getChatFromVods(link string) ([]string, error) {
	videoId := extractId(link)
	u := "https://api.twitch.tv/v5/videos/" + videoId + "/comments?cursor="
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))
	var vodsChat VodsChat
	err := requestJSON(req, 10, &vodsChat)
	if err != nil {
		return nil, err
	}
	messages := vodsChat.parse()
	fmt.Println(messages)
	for vodsChat.Next != "" {
		req.URL, _ = url.Parse(u + vodsChat.Next)
		vodsChat = VodsChat{}
		err := requestJSON(req, 10, &vodsChat)
		if err != nil {
			return nil, err
		}
		newMessages := vodsChat.parse()
		fmt.Println(newMessages)
		messages = append(messages, newMessages...)
	}
	return messages, nil
}

func asciifyRequest(url string, width int, reverse bool, thMult float32) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	img, _, err := image.Decode(res.Body)
	if err != nil {
		log.Println(err)
	}
	return Braille(img, width, reverse, thMult), nil
}

func parseCommand(str string, botInstances map[string]*Bot, console *Console) {
	commandsChain := strings.Split(str, "|")
	for _, s := range commandsChain {
		s = strings.Trim(s, " ")
		args := strings.Split(s, " ")
		command := args[0]
		args = args[1:]

		switch command {
		case "connect":
			if len(args) != 1 {
				fmt.Println("Provide channel name")
				break
			}
			if _, ok := botInstances[args[0]]; !ok {
				go startBot(args[0], botInstances)
			}
			console.currentChannel = args[0]
		case "find":
			if len(args) != 1 {
				fmt.Println("Provide channel name")
				break
			}
			if args[0] == "list" {
				rows := personsList("%")
				for i := range rows {
					fmt.Print(rows[i] + " ")
				}
				fmt.Println("")
			} else {
				findPerson(args[0])
			}
		case "asciify":
			//fmt.Println(asciify(args))
		case "disconnect":
			if len(args) != 1 {
				fmt.Println("Provide channel name")
				break
			}
			if _, ok := botInstances[args[0]]; !ok {
				fmt.Println("No such channel")
				break
			}
			botInstances[args[0]].Disconnect()
			delete(botInstances, args[0])
			console.currentChannel = "#"
		case "change":
			if len(args) != 3 {
				fmt.Println("Provide valid args")
				break
			}
			if _, ok := botInstances[args[0]]; ok {
				botInstances[args[0]].changeAuthority(args[1], args[2])
			} else {
				fmt.Println("Provide valid channel name to which bot is currently connected")
			}
		case "clear":
			if len(args) == 0 {
				fmt.Print("\033[H\033[J")
				break
			}
			if args[0] == "buffer" {
				console.commandsBuffer.Clear()
			}
		case "loademotes":
			if len(botInstances) == 0 {
				fmt.Println("You are not connected to any channel")
				break
			}
			go botInstances[console.currentChannel].updateEmotes()
		case "send":
			if len(args) < 1 {
				fmt.Println("write message")
				break
			}
			if console.currentChannel == "#" {
				fmt.Println("connect to chat")
				break
			}
			str := args[0]
			for i := 1; i < len(args); i++ {
				str += " " + args[i]
			}
			botInstances[console.currentChannel].SendMessage(str)
		case "markov":
			if len(args) != 1 {
				log.Println("something went wrong")
				break
			}
			msg, err := Markov(args[0])
			if err != nil {
				log.Println(err)
				break
			}
			fmt.Println(msg)
		case "loadcomments":
			if len(args) != 1 {
				fmt.Println("something went wrong")
				break
			}
			var err error
			console.comments, err = getChatFromVods(args[0])
			if err != nil {
				log.Println(err)
			}
		case "sortcomments":
			if console.comments == nil {
				fmt.Println("load some comments")
				break
			}
			var timeStart time.Time
			var timeEnd time.Time
			commentsArgs := strings.Split(s[strings.Index(s, " ")+1:], ",")
			length := len(commentsArgs)
			if length == 1 {
				timeEnd = time.Now()
			} else if length == 3 {
				var err error
				timeStart, timeEnd, err = parseLogTime(commentsArgs[1], commentsArgs[2])
				if err != nil {
					log.Println(err)
					break
				}
			} else {
				break
			}
			username := commentsArgs[0]
			for _, comment := range console.comments {
				str, err := logsParse(comment, "", username, timeStart, timeEnd)
				if err != nil {
					continue
				}
				fmt.Print(str)
			}
		case "interactivesort":
			interactiveSort()
		case "savechat":
			if console.comments == nil {
				fmt.Println("load some comments")
				break
			}
			file, err := os.OpenFile("vod.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				log.Println(err)
				break
			}
			w := bufio.NewWriter(file)
			for _, comment := range console.comments {
				w.WriteString(comment)
			}
			defer file.Close()
		case "clearcomments":
			if console.comments == nil {
				fmt.Println("load some comments")
				break
			}
			console.comments = nil
		case "searchtrack":
			track, err := searchTrack(s[strings.Index(s, " ")+1:])
			if err != nil {
				fmt.Println(err)
				break
			}
			addToPlaylist(track.Tracks.Items[0].URI)
		case "currenttrack":
			fmt.Println(getCurrentTrack())
		case "nexttrack":
			skipToNextTrack()
		}
	}
}

type Console struct {
	commandsBuffer Buffer
	currentChannel string
	comments       []string
}

func main() {
	var console Console
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	console.currentChannel = "#"
	botInstaces := make(map[string]*Bot)
	setTerm()
	coreRenderer := CoreRenderer{currentChannel: &console.currentChannel}
	for {
		args, status := console.processConsole(coreRenderer)
		switch status {
		case ENTER:
			parseCommand(args, botInstaces, &console)
		}
	}
}
