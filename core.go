package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"time"
)

type Bot struct {
	Channel string
	Name    string
	Port    string
	OAuth   string
	Server  string
	Conn    net.Conn
	File    *os.File
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
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		fmt.Println(line)
		messages := strings.Split(line, "\r\n")
		for _, msg := range messages {
			if strings.Contains(msg, "PRIVMSG") {
				userdata := strings.Split(msg, ".tmi.twitch.tv PRIVMSG "+bot.Channel)
				username := strings.Split(userdata[0], "@")[1]
				usermessage := strings.Replace(userdata[1], " :", "", 1)
				userMessageCommand := strings.Split(usermessage, " ")[0]
				if username == "funwayz" && userMessageCommand[0:1] == "!" {
					fmt.Println("test2")
					go bot.Commands(userMessageCommand)
				}
				fmt.Fprintf(w, "[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"), username, usermessage)
			} else if strings.Contains(msg, "PING") {
				bot.Pong(msg)
				fmt.Fprintln(w, msg)
			} else {
				fmt.Fprintln(w, msg)
			}
			w.Flush()
		}
		defer wg.Done()
	}
}

func (bot *Bot) Commands(command string) {
	if strings.Contains(command, "!logs(") {
		t := strings.Split(command, "(")[1]
		name := strings.Split(t, ")")[0]
		fmt.Println(name)
		r := bufio.NewScanner(bot.File)
		for r.Scan() {
			if strings.Contains(r.Text(), name) {
				fmt.Fprintf(bot.Conn, "PRIVMSG %s :%s\r\n", bot.Channel, r.Text())
			}
		}
	}
}

func main() {
	wg := new(sync.WaitGroup)
	logfile, err := os.OpenFile("test.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer logfile.Close()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	channel := scanner.Text()
	bot := Bot{
		Channel: channel,
		Name:    "funwayz",
		Port:    "6667",
		OAuth:   os.Getenv("TWITCH_OAUTH_ENV"),
		Server:  "irc.twitch.tv",
		File:    logfile,
		Conn:    nil,
	}
	bot.Connect(wg)
	wg.Wait()
}
