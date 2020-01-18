package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"
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
	Authority map[string]string
	Status    string
	Utils     Utils
}

// connects to twitch chat
func (bot *Bot) Connect(wg *sync.WaitGroup) {
	var err error
	bot.Conn, err = net.Dial("tcp", bot.Server+":"+bot.Port)
	if err != nil {
		fmt.Printf("Unable to connect!")
	}
	fmt.Fprintf(bot.Conn, "PASS %s\r\n", bot.OAuth)
	fmt.Fprintf(bot.Conn, "NICK %s\r\n", bot.Name)
	fmt.Fprintf(bot.Conn, "JOIN %s\r\n", bot.Channel)
	wg.Add(1)
	go bot.reader(wg)
}

func (bot *Bot) Pong(line string) {
	pong := strings.Split(line, "PING")
	fmt.Fprintf(bot.Conn, "PONG %s\r\n", pong[1])
}

// reader and parser
func (bot *Bot) reader(wg *sync.WaitGroup) {
	tp := textproto.NewReader(bufio.NewReader(bot.Conn))
	w := bufio.NewWriter(bot.File)
	pasteFile, err := os.OpenFile("paste.txt", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Println(err)
	}
	pasteWriter := bufio.NewWriter(pasteFile)
	for {
		line, err := tp.ReadLine()
		if err != nil {
			fmt.Println(err)
			break
		}
		// parsing chat
		if strings.Contains(line, "PRIVMSG") {
			userdata := strings.Split(line, ".tmi.twitch.tv PRIVMSG "+bot.Channel)
			username := strings.Split(userdata[0], "@")[1]
			usermessage := strings.Replace(userdata[1], " :", "", 1)
			fmt.Fprintf(w, "[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"), username, usermessage)
			w.Flush()
			messageLength := len(usermessage)
			if bot.Status == "smartvote" && messageLength == 1 {
				if _, ok := bot.Utils.SmartVote.Options[usermessage]; ok {
					if _, ok := bot.Utils.SmartVote.Votes[username]; !ok {
						bot.Utils.SmartVote.Options[usermessage]++
						bot.Utils.SmartVote.Votes[username] = usermessage
					}
				}
			}
			if messageLength >= 200 && messageLength <= 500 {
				fmt.Fprintf(pasteWriter, "%s\n\n", usermessage)
			}

			if bot.Authority[username] == "top" && usermessage[0:1] == "!" {
				go bot.Commands(usermessage, username)
			}
		} else if strings.Contains(line, "PING") { // response to keep connection alive
			bot.Pong(line)
			fmt.Fprintln(w, line)
			w.Flush()
		} else {
			fmt.Fprintln(w, line)
			w.Flush()
		}
		defer wg.Done()
	}
}

// chat commands
func (bot *Bot) Commands(command string, username string) {
	var cmd Command
	var err error
	cmd.Parse(command)
	switch cmd.Name {
	// !logs(username, timeStart, timeEnd)
	case "logs":
		err = bot.LogsCommand(cmd.Params)
	// !smartvote(lowerBound, upperBound)
	case "smartvote":
		err = bot.SmartVoteCommand(cmd.Params)
	// !voteoptions
	case "voteoptions":
		err = bot.VoteOptionsCommand()
	// !stopvote
	case "stopvote":
		bot.Status = "Running"
	}
	if err != nil {
		bot.Status = "Running"
		fmt.Println(err)
	}
}

func authorityInit() map[string]string {
	file, err := ioutil.ReadFile("authority.txt")
	if err != nil {
		fmt.Println(err)
	}
	var m map[string]string
	if err = json.Unmarshal(file, &m); err != nil {
		fmt.Println(err)
	}
	return m
}

func main() {
	wg := new(sync.WaitGroup)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	channel := scanner.Text()
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
	bot.Connect(wg)
	wg.Wait()
}
