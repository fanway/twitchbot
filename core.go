package main

import (
	"bufio"
	"fmt"
	"io"
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
}

// connects to twitch chat
func (bot *Bot) Connect(wg *sync.WaitGroup) {
	conn, err := net.Dial("tcp", bot.Server+":"+bot.Port)
	if err != nil {
		fmt.Printf("Unable to connect!")
	}
	fmt.Fprintf(conn, "PASS %s\r\n", bot.OAuth)
	fmt.Fprintf(conn, "NICK %s\r\n", bot.Name)
	fmt.Fprintf(conn, "JOIN %s\r\n", bot.Channel)
	wg.Add(1)
	go bot.reader(conn, wg)
}

// reader and parser
func (bot *Bot) reader(reader io.Reader, wg *sync.WaitGroup) {
	tp := textproto.NewReader(bufio.NewReader(reader))
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return
		}
		messages := strings.Split(line, "\r\n")
		for _, msg := range messages {
			if strings.Contains(msg, "PRIVMSG") {
				userdata := strings.Split(msg, ".tmi.twitch.tv PRIVMSG "+bot.Channel)
				username := strings.Split(userdata[0], "@")[1]
				usermessage := strings.Replace(userdata[1], " :", "", 1)
				fmt.Printf("[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"), username, usermessage)
			} else {
				fmt.Println(msg)
			}
		}
		defer wg.Done()
	}
}

func main() {
	wg := new(sync.WaitGroup)
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	channel := scanner.Text()
	bot := Bot{
		Channel: channel,
		Name:    "funwayz",
		Port:    "6667",
		OAuth:   os.Getenv("TWITCH_OAUTH_ENV"),
		Server:  "irc.twitch.tv",
	}
	bot.Connect(wg)
	wg.Wait()
}
