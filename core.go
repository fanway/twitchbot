package main

import (
	"bufio"
	"fmt"
	"io"
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
			panic(err)
		}
		if strings.Contains(line, "PRIVMSG") {
			userdata := strings.Split(line, ".tmi.twitch.tv PRIVMSG "+bot.Channel)
			username := strings.Split(userdata[0], "@")[1]
			usermessage := strings.Replace(userdata[1], " :", "", 1)
			userMessageCommand := strings.Split(usermessage, " ")[0]
			fmt.Fprintf(w, "[%s] %s: %s\n", time.Now().Format("2006-01-02 15:04:05 -0700 MST"), username, usermessage)
			w.Flush()
			if username == "funwayz" && userMessageCommand[0:1] == "!" {
				go bot.Commands(userMessageCommand)
			}
		} else if strings.Contains(line, "PING") {
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

func (bot *Bot) Commands(command string) {
	if strings.Contains(command, "!logs(") {
		t := strings.Split(command, "(")[1]
		name := strings.Split(t, ")")[0]
		offset, err := bot.File.Seek(0, io.SeekStart)
		if err != nil {
			panic(err)
		}
		r := bufio.NewScanner(bot.File)
		fmt.Println(offset)
		var str string
		for r.Scan() {
			str = r.Text()
			if strings.Count(str, ":") > 2 {
				if strings.Contains(strings.Split(str, ":")[2], name) {
					fmt.Fprintf(bot.Conn, "PRIVMSG %s :%s\r\n", bot.Channel, str)
				}
			}
			if err := r.Err(); err != nil {
				fmt.Println(err)
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
