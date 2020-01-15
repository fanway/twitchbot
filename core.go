package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/textproto"
	"os"
	"sort"
	"strconv"
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
			if bot.Status == "smartvote" && len(usermessage) == 1 {
				if _, ok := bot.Utils.SmartVote.Options[usermessage]; ok {
					if _, ok := bot.Utils.SmartVote.Votes[username]; !ok {
						bot.Utils.SmartVote.Options[usermessage]++
						bot.Utils.SmartVote.Votes[username] = usermessage
					}
				}
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
	// !logs(username, timeStart, timeEnd)
	if strings.Contains(command, "!logs(") {
		startIdx := strings.Index(command, "(") + 1
		endIdx := strings.Index(command, ")")
		if startIdx == -1 || endIdx == -1 {
			fmt.Println("!logs: wrong syntax")
			return
		}
		params := command[startIdx:endIdx]
		// username, timeStart, timeEnd (utt)
		utt := strings.Split(params, ",")
		if len(utt) < 3 {
			fmt.Println("!logs: wrong amount of params")
			return
		}
		layout := "2006-01-02 15:04:05 -0700 MST"
		username := utt[0]
		timeStart, err := time.Parse(layout, "2019-"+strings.TrimSpace(utt[1])+":00 +0300 MSK")
		if err != nil {
			fmt.Println(err)
			return
		}
		timeEnd, err := time.Parse(layout, "2019-"+strings.TrimSpace(utt[2])+":00 +0300 MSK")
		if err != nil {
			fmt.Println(err)
			return
		}
		if _, err = bot.File.Seek(0, io.SeekStart); err != nil {
			panic(err)
		}
		r := bufio.NewScanner(bot.File)
		var str string
		var tt string
		for r.Scan() {
			str = r.Text()
			if strings.Contains(str, "[") {
				tt = strings.Split(strings.Split(str, "[")[1], "]")[0]
				timeq, err := time.Parse(layout, tt)
				if err != nil {
					fmt.Println(err)
					break
				}
				if timeq.Before(timeEnd) && timeq.After(timeStart) {
					if strings.Count(str, ":") > 2 {
						if strings.Contains(strings.Split(str, ":")[2], username) || username == "all" {
							fmt.Fprintf(bot.Conn, "PRIVMSG %s :%s\r\n", bot.Channel, str)
						}
					}
				}
			}
			if err := r.Err(); err != nil {
				fmt.Println(err)
			}
		}
	}
	// !smartvote 1-2
	if strings.Contains(command, "!smartvote") {
		bot.Utils.SmartVote.Options = make(map[string]int)
		bot.Utils.SmartVote.Votes = make(map[string]string)
		bot.Status = "smartvote"
		split := strings.Split(command, " ")
		if len(split) < 2 {
			fmt.Println("Not enough args")
			bot.Status = "Running"
			return
		}

		str := "GOLOSOVANIE"
		fmt.Fprintf(bot.Conn, "PRIVMSG %s :%s\r\n", bot.Channel, str)

		r := strings.Split(split[1], "-")
		lowerBound, err := strconv.Atoi(r[0])
		if err != nil {
			fmt.Println(err)
			bot.Status = "Running"
			return
		}

		upperBound, err := strconv.Atoi(r[1])
		if err != nil {
			fmt.Println(err)
			bot.Status = "Running"
			return
		}

		for i := lowerBound; i <= upperBound; i++ {
			voteStr := strconv.Itoa(i)
			bot.Utils.SmartVote.Options[voteStr] = 0
		}

	}
	if strings.Contains(command, "!stopvote") {
		bot.Status = "Running"
	}
	if strings.Contains(command, "!voteoptions") {
		length := len(bot.Utils.SmartVote.Options)
		keys := make([]string, length)
		i := 0
		for k := range bot.Utils.SmartVote.Options {
			keys[i] = k
			i++
		}
		sort.Strings(keys)
		var str string
		total := len(bot.Utils.SmartVote.Votes)
		str = fmt.Sprintf("Total votes %d: ", total)
		percent := float32(bot.Utils.SmartVote.Options[keys[0]]) / float32(total) * 100
		str += fmt.Sprintf("%s: %.1f%%(%d)", keys[0], percent, bot.Utils.SmartVote.Options[keys[0]])
		for i := 1; i < length; i++ {
			percent = float32(bot.Utils.SmartVote.Options[keys[i]]) / float32(total) * 100
			str += fmt.Sprintf(", %s: %.1f%%(%d)", keys[i], percent, bot.Utils.SmartVote.Options[keys[i]])
		}

		fmt.Fprintf(bot.Conn, "PRIVMSG %s :%s\r\n", bot.Channel, str)
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
	fmt.Println(m["top"])
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
