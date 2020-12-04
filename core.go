package main

import (
	"bufio"
	"fmt"
	"log"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"
	"twitchStats/logsparser"
	"twitchStats/markov"
	"twitchStats/spotify"
	"twitchStats/terminal"

	_ "github.com/mattn/go-sqlite3"
	//"io/ioutil"
)

func parseCommand(str string, botInstances map[string]*Bot) {
	commandsChain := strings.Split(str, "|")
	for _, s := range commandsChain {
		s = strings.Trim(s, " ")
		args := strings.Split(s, " ")
		command := args[0]
		args = args[1:]

		switch command {
		case "connect":
			if len(args) != 1 {
				terminal.Output.Println("Provide channel name")
				break
			}
			if _, ok := botInstances[args[0]]; !ok {
				go startBot(args[0], botInstances)
			}
			terminal.Output.CurrentChannel = args[0]
		case "find":
			if len(args) != 1 {
				terminal.Output.Println("Provide channel name")
				break
			}
			if args[0] == "list" {
				rows := terminal.PersonsList("%")
				for i := range rows {
					terminal.Output.Print(rows[i] + " ")
				}
				terminal.Output.Println("")
			} else {
				terminal.FindPerson(args[0])
			}
		case "asciify":
			//terminal.Output.Println(asciify(args))
		case "disconnect":
			if len(args) != 1 {
				terminal.Output.Println("Provide channel name")
				break
			}
			if _, ok := botInstances[args[0]]; !ok {
				terminal.Output.Println("No such channel")
				break
			}
			botInstances[args[0]].Disconnect()
			delete(botInstances, args[0])
			terminal.Output.CurrentChannel = "#"
		case "change":
			if len(args) != 3 {
				terminal.Output.Println("Provide valid args")
				break
			}
			if _, ok := botInstances[args[0]]; ok {
				botInstances[args[0]].changeAuthority(args[1], args[2])
			} else {
				terminal.Output.Println("Provide valid channel name to which bot is currently connected")
			}
		case "clear":
			if len(args) == 0 {
				terminal.Output.Print("\033[H\033[J")
				break
			}
			if args[0] == "buffer" {
				terminal.Output.CommandsBuffer.Clear()
			}
		case "loademotes":
			if len(botInstances) == 0 {
				terminal.Output.Println("You are not connected to any channel")
				break
			}
			go botInstances[terminal.Output.CurrentChannel].updateEmotes()
		case "send":
			if len(args) < 1 {
				terminal.Output.Println("write message")
				break
			}
			if terminal.Output.CurrentChannel == "#" {
				terminal.Output.Println("connect to chat")
				break
			}
			str := args[0]
			for i := 1; i < len(args); i++ {
				str += " " + args[i]
			}
			botInstances[terminal.Output.CurrentChannel].SendMessage(str)
		case "markov":
			if len(args) != 1 {
				terminal.Output.Log("something went wrong")
				break
			}
			msg, err := markov.Markov(args[0])
			if err != nil {
				terminal.Output.Log(err)
				break
			}
			terminal.Output.Println(msg)
		case "spam":
			bot := botInstances[terminal.Output.CurrentChannel]
			duration := time.Duration(90) * time.Second
			switch len(args) {
			case 0:
				bot.Status = "Running"
				bot.Spam.Clear()
				break
			case 2:
				var err error
				duration, err = time.ParseDuration(args[1])
				if err != nil {
					terminal.Output.Log(err)
					break
				}
			}
			bot.Spam.Add(args[0])
			bot.SpamHistory(args[0], duration)
			bot.Status = "SpamAttack"
		case "loadcomments":
			if len(args) != 1 {
				terminal.Output.Println("something went wrong")
				break
			}
			var err error
			terminal.Output.Comments, err = terminal.GetChatFromVods(args[0])
			if err != nil {
				terminal.Output.Log(err)
			}
		case "sortcomments":
			if terminal.Output.Comments == nil {
				terminal.Output.Println("load some comments")
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
				timeStart, timeEnd, err = logsparser.ParseTime(commentsArgs[1], commentsArgs[2])
				if err != nil {
					terminal.Output.Log(err)
					break
				}
			} else {
				break
			}
			username := commentsArgs[0]
			for _, comment := range terminal.Output.Comments {
				parsedStr, err := logsparser.Parse(comment, "", username, timeStart, timeEnd)
				if err != nil {
					continue
				}
				terminal.Output.Print(fmt.Sprintf("[%s] %s: %s", parsedStr[1], parsedStr[2], parsedStr[3]))
			}
		case "interactivesort":
			terminal.InteractiveSort()
		case "savechat":
			if terminal.Output.Comments == nil {
				terminal.Output.Println("load some comments")
				break
			}
			file, err := os.OpenFile("vod.log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
			if err != nil {
				terminal.Output.Log(err)
				break
			}
			w := bufio.NewWriter(file)
			for _, comment := range terminal.Output.Comments {
				w.WriteString(comment)
			}
			defer file.Close()
		case "clearcomments":
			if terminal.Output.Comments == nil {
				terminal.Output.Println("load some comments")
				break
			}
			terminal.Output.Comments = nil
		case "searchtrack":
			track, err := spotify.SearchTrack(s[strings.Index(s, " ")+1:])
			if err != nil {
				terminal.Output.Println(err)
				break
			}
			spotify.AddToPlaylist(track.Tracks.Items[0].URI)
		case "currenttrack":
			track, err := spotify.GetCurrentTrack()
			if err != nil {
				terminal.Output.Log(err)
				break
			}
			terminal.Output.Println(track)
		case "nexttrack":
			spotify.SkipToNextTrack()
		case "changestatus":
			if len(args) != 1 {
				terminal.Output.Println("something went wrong")
				break
			}
			bot := botInstances[terminal.Output.CurrentChannel]
			bot.Status = args[0]
		case "crossfollow":
			if len(args) != 2 {
				terminal.Output.Println("not enough args")
				break
			}
			terminal.Output.Println(terminal.CrossFollow(args[0], args[1]))
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	terminal.Output.CurrentChannel = "#"
	botInstaces := make(map[string]*Bot)
	terminal.SetTerm()
	coreRenderer := terminal.CoreRenderer{CurrentChannel: &terminal.Output.CurrentChannel}
	terminal.Output.Renderer = &coreRenderer
	for {
		args, status := terminal.Output.ProcessConsole()
		switch status {
		case terminal.ENTER:
			parseCommand(args, botInstaces)
		}
	}
}
