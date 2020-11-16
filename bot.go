package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	pb "twitchStats/commands/pb"
	"twitchStats/database"
	"twitchStats/logsparser"
	"twitchStats/request"
	"twitchStats/terminal"

	"google.golang.org/grpc"
)

const (
	LOW = iota
	MIDDLE
	TOP
)

const (
	BotName = "funwayz"
	Port    = "6667"
	Server  = "irc.twitch.tv"
)

type Bot struct {
	Channel     string
	OAuth       string
	Conn        net.Conn
	File        *os.File
	StopChannel chan struct{}
	Authority   map[string]int
	Status      string
	Warn        Warn
	BadWords    map[string]struct{}
	Spam        Spam
	GrpcClient  pb.CommandsClient
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

// connects to twitch chat
func (bot *Bot) Connect() {
	var err error
	bot.Conn, err = net.Dial("tcp", Server+":"+Port)
	if err != nil {
		fmt.Printf("Unable to connect!")
	}
	fmt.Fprintf(bot.Conn, "CAP REQ :twitch.tv/tags\r\n")
	fmt.Fprintf(bot.Conn, "PASS %s\r\n", bot.OAuth)
	fmt.Fprintf(bot.Conn, "NICK %s\r\n", BotName)
	fmt.Fprintf(bot.Conn, "JOIN %s\r\n", bot.Channel)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	opts := []grpc.DialOption{grpc.WithInsecure()}
	grpcConn, err := grpc.Dial("localhost:3434", opts...)
	if err != nil {
		fmt.Println("Unable to connect to grpc")
	}
	bot.GrpcClient = pb.NewCommandsClient(grpcConn)
	go bot.checkReminders()
	go bot.reader(wg)
	terminal.Output.Println("connected to " + bot.Channel)
	wg.Wait()
}

func (bot *Bot) Disconnect() {
	close(bot.StopChannel)
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
	logChan := make(chan Message)
	defer wg.Done()
	go bot.logsWriter(logChan)
	for {
		select {
		case <-bot.StopChannel:
			return
		default:
			line, err := tp.ReadLine()
			if err != nil {
				terminal.Output.Log(err)
				return
			}
			// parsing chat
			go bot.parseChat(line, logChan)
		}
	}
}

type Message struct {
	Username string
	Text     string
	Emotes   string
	ID       string
}

func (bot *Bot) logsWriter(logChan <-chan Message) {
	w := bufio.NewWriter(bot.File)
	for {
		select {
		case message := <-logChan:
			fmt.Fprintf(w, "[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"), message.Username, message.Text)
			w.Flush()
		case <-bot.StopChannel:
			return
		}
	}
}

type Warning struct {
	Reason      string
	TimeCreated time.Time
}

type Warn struct {
	sync.Mutex
	Warnings map[string]*[]Warning
}

func (bot *Bot) timeout(username, reason string, seconds int) {
	bot.SendMessage("/timeout " + username + " " + strconv.Itoa(seconds))
	bot.SendMessage("@" + username + " " + reason)
}

func (bot *Bot) ban(username string) {
	bot.SendMessage("/ban " + username)
}

func (bot *Bot) warning(username, id, reason string, seconds int) {
	bot.Warn.Lock()
	defer bot.Warn.Unlock()
	warnings, ok := bot.Warn.Warnings[username]
	if !ok {
		warnings = &[]Warning{}
		bot.Warn.Warnings[username] = warnings
	}
	length := len(*warnings)
	if length == 2 {
		bot.timeout(username, "3rd warning", 86400)
		delete(bot.Warn.Warnings, username)
		return
	}
	for i, _ := range *warnings {
		if time.Since((*warnings)[i].TimeCreated)*time.Second > 1800 {
			(*warnings)[i] = (*warnings)[length]
			*warnings = (*warnings)[:length-1]
		}
	}
	*warnings = append(*warnings, Warning{Reason: reason, TimeCreated: time.Now()})
	bot.timeout(username, reason, seconds)
}

func (bot *Bot) checkMessage(msg *Message) bool {
	split := strings.Split(msg.Text, " ")
	if len(split) > 1 {
		split = append(split, msg.Text)
	}
	for i, _ := range split {
		if _, ok := bot.BadWords[split[i]]; ok {
			bot.warning(msg.Username, msg.ID, "Warning: Usage of explicit language", 300)
			return true
		}
	}
	return false
}

func (bot *Bot) checkReminders() {
	ticker := time.NewTicker(10 * time.Second)
	for {
		stream, err := bot.GrpcClient.ParseAndExec(context.Background(), &pb.Message{Channel: bot.Channel, Text: "!fetchreminder", Status: bot.Status, Level: 2})
		if err != nil {
			terminal.Output.Log(err)
			return
		}
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				terminal.Output.Log(err)
				break
			}
			bot.SendMessage(in.Text)
		}
		<-ticker.C
	}
}

func (bot *Bot) checkAfk(msg *Message) {
	re := regexp.MustCompile(`@(\w+)`)
	match := re.FindStringSubmatch(msg.Text)
	var cmd string
	if len(match) == 0 {
		cmd = "!checkafk"
	} else {
		cmd = "!checkafk " + match[1]
	}
	stream, err := bot.GrpcClient.ParseAndExec(context.Background(), &pb.Message{Channel: bot.Channel, Text: cmd, Username: msg.Username, Status: bot.Status, Level: 2})
	if err != nil {
		terminal.Output.Log(err)
		return
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			terminal.Output.Log(err)
			break
		}
		bot.Status = in.Status
		bot.SendMessage(in.Text)
	}
}
func (bot *Bot) pasteWriter(msg *Message) {
	pasteFile, err := os.OpenFile("paste.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		terminal.Output.Log(err)
	}
	defer pasteFile.Close()
	pasteWriter := bufio.NewWriter(pasteFile)
	_, err = fmt.Fprintf(pasteWriter, "%s\n\n", msg.Text)
	if err != nil {
		terminal.Output.Log(err)
	}
	pasteWriter.Flush()
}

func (bot *Bot) SpamHistory(spamMsg string, duration time.Duration) error {
	if _, err := bot.File.Seek(0, io.SeekStart); err != nil {
		panic(err)
	}
	r := bufio.NewScanner(bot.File)
	timeEnd := time.Now()
	timeStart := timeEnd.Add(-duration)
	for r.Scan() {
		str := r.Text()
		username, err := logsparser.Parse(str, spamMsg, "all", timeStart, timeEnd)
		if err != nil {
			continue
		}
		bot.ban(username)
		if err := r.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (spam *Spam) Add(spamMsg string) {
	spam.Lock()
	spam.Messages = append(spam.Messages, spamMsg)
	spam.Unlock()
}

func (spam *Spam) Clear() {
	spam.Lock()
	spam.Messages = nil
	spam.Unlock()
}

type Spam struct {
	sync.RWMutex
	Messages []string
}

func (bot *Bot) parseChat(line string, logChan chan<- Message) {
	if strings.Contains(line, "PRIVMSG") {
		re := regexp.MustCompile(`emotes=(.*?);|@(.*?)\.tmi\.twitch\.tv|PRIVMSG.*?:(.*)|id=(.*?);`)
		match := re.FindAllStringSubmatch(line[1:], -1)
		message := Message{
			Emotes:   match[0][1],
			ID:       match[1][4],
			Username: match[4][2],
			Text:     match[5][3],
		}
		logChan <- message
		messageLength := len(message.Text)
		switch bot.Status {
		case "Running":
			if messageLength >= 300 && messageLength <= 2000 {
				go bot.pasteWriter(&message)
			}
			if bot.checkMessage(&message) {
				return
			}
			bot.checkAfk(&message)
			if message.Text[0] == '!' {
				bot.processCommands(&message)
			}
		case "Smartvote":
			if messageLength == 1 {
				message.Text = "!vote " + message.Text
				bot.processCommands(&message)
			}
		case "SpamAttack":
			bot.Spam.RLock()
			for i, _ := range bot.Spam.Messages {
				if strings.Contains(message.Text, bot.Spam.Messages[i]) {
					bot.ban(message.Username)
				}
			}
			bot.Spam.RUnlock()
		}

	} else if strings.HasPrefix(line, "PING") { // response to keep connection alive
		bot.Pong(line)
	}
}

// chat commands
func (bot *Bot) processCommands(message *Message) {
	level := bot.Authority[message.Username]
	stream, err := bot.GrpcClient.ParseAndExec(context.Background(), &pb.Message{
		Channel:  bot.Channel,
		Username: message.Username,
		Text:     message.Text,
		Emotes:   message.Emotes,
		Id:       message.ID,
		Level:    int32(level),
		Status:   bot.Status,
	})
	if err != nil {
		terminal.Output.Log(err)
		return
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			terminal.Output.Log(err)
			break
		}
		bot.Status = in.Status
		bot.SendMessage(in.Text)
	}
}

func initBadWords() map[string]struct{} {
	file, err := ioutil.ReadFile("badwords.txt")
	if err != nil {
		terminal.Output.Log(err)
	}
	var m map[string]struct{}
	if err = json.Unmarshal(file, &m); err != nil {
		terminal.Output.Log(err)
	}
	return m
}

func initAuthority() map[string]int {
	file, err := ioutil.ReadFile("authority.txt")
	if err != nil {
		terminal.Output.Log(err)
	}
	var m map[string]string
	if err = json.Unmarshal(file, &m); err != nil {
		terminal.Output.Log(err)
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

func (bot *Bot) changeAuthority(username, level string) {
	file, err := ioutil.ReadFile("authority.txt")
	if err != nil {
		terminal.Output.Log(err)
	}
	var m map[string]string
	if err = json.Unmarshal(file, &m); err != nil {
		terminal.Output.Log(err)
	}
	m[username] = level
	newAuthority, err := json.Marshal(m)
	if err != nil {
		terminal.Output.Log(err)
		return
	}
	err = ioutil.WriteFile("authority.txt", newAuthority, 0644)
	if err != nil {
		terminal.Output.Log(err)
		return
	}
	bot.Authority = initAuthority()
}

func (bot *Bot) updateEmotes() {
	db := database.Connect()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		terminal.Output.Log(err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS ffzbttv(url TEXT NOT NULL, code TEXT NOT NULL, UNIQUE (url) ON CONFLICT REPLACE);")
	if err != nil {
		terminal.Output.Log(err)
	}
	ffzUrl := "https://api.frankerfacez.com/v1/room/" + bot.Channel[1:]
	var ffz map[string]interface{}
	req, _ := http.NewRequest("GET", ffzUrl, nil)
	err = request.JSON(req, 10, &ffz)
	if err != nil {
		terminal.Output.Log(err)
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
			terminal.Output.Log(err)
		}
	}

	bttvUrl := "https://api.betterttv.net/3/cached/users/twitch/" + strconv.FormatInt(twitchID, 10)
	var bttv Bttv
	req, _ = http.NewRequest("GET", bttvUrl, nil)
	err = request.JSON(req, 10, &bttv)
	if err != nil {
		terminal.Output.Log(err)
	}
	cdnUrl := "https://cdn.betterttv.net/emote/"
	for _, u := range bttv.SharedEmotes {
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", cdnUrl+u.ID+"/3x", u.Code)
		if err != nil {
			terminal.Output.Log(err)
		}
	}
	for _, u := range bttv.ChannelEmotes {
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", cdnUrl+u.ID+"/3x", u.Code)
		if err != nil {
			terminal.Output.Log(err)
		}
	}

	twitchUrl := "https://api.twitch.tv/api/channels/" + bot.Channel[1:] + "/product"
	req, _ = http.NewRequest("GET", twitchUrl, nil)
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID_"))
	var tempTwitch map[string]json.RawMessage
	err = request.JSON(req, 10, &tempTwitch)
	if err != nil {
		terminal.Output.Log(err)
		return
	}
	var twitch TwitchEmotes
	if err := json.Unmarshal(tempTwitch["emoticons"], &twitch); err != nil {
		terminal.Output.Log(err)
		return
	}

	for _, v := range twitch {
		url := strings.Replace(v.URL, "/1.0", "/3.0", 1)
		_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", url, v.Regex)
		if err != nil {
			terminal.Output.Log(err)
		}
	}

	tx.Commit()
}

func startBot(channel string, botInstances map[string]*Bot) {
	logfile, err := os.OpenFile(channel+".log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()
	bot := Bot{
		Channel:     channel,
		File:        logfile,
		Conn:        nil,
		OAuth:       os.Getenv("TWITCH_OAUTH_ENV"),
		StopChannel: make(chan struct{}),
		BadWords:    initBadWords(),
		Authority:   initAuthority(),
		Status:      "Running",
		Warn:        Warn{Warnings: make(map[string]*[]Warning)},
	}
	botInstances[channel] = &bot
	bot.Connect()
}
