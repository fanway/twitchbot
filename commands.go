package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Command struct {
	Name      string
	Params    *Message
	LastUsage time.Time
	Cd        int
	Level     int
	Handler   func(*Message) error
}

func checkForUrl(url string) string {
	if strings.HasPrefix(url, "https://") &&
		(strings.HasSuffix(url, ".jpeg") || strings.HasSuffix(url, ".jpg") || strings.HasSuffix(url, ".png")) {
		return url
	}
	return ""
}

func (bot *Bot) parseCommand(message *Message) (*Command, error) {
	splitIndex := strings.Index(message.Text, " ")
	if cmd, ok := bot.Commands[message.Text[1:splitIndex]]; ok {
		cmd.Params = message
		return cmd, nil
	}
	return nil, errors.New("could not parse command")
}

func (bot *Bot) initCommands() {
	bot.Commands = map[string]*Command{
		// !logs username, timeStart, timeEnd
		"logs": &Command{
			Name:    "logs",
			Cd:      60,
			Level:   TOP,
			Handler: bot.LogsCommand,
		},
		// !smartvote lowerBound, upperBound
		"smartvote": &Command{
			Name:    "smartvote",
			Cd:      30,
			Level:   TOP,
			Handler: bot.SmartVoteCommand,
		},
		// !stopvote
		"stopvote": &Command{
			Name:    "stopvote",
			Cd:      15,
			Level:   TOP,
			Handler: bot.StopVoteCommand,
		},
		// !voteoptions
		"voteoptions": &Command{
			Name:    "voteoptions",
			Cd:      5,
			Level:   MIDDLE,
			Handler: bot.VoteOptionsCommand,
		},
		// !asciify <emote>
		"asciify": &Command{
			Name:    "asciify",
			Cd:      10,
			Level:   MIDDLE,
			Handler: bot.Asciify,
		},
		// !asciify <emote>
		"asciify~": &Command{
			Name:    "asciify~",
			Cd:      10,
			Level:   MIDDLE,
			Handler: bot.Asciify,
		},
		// !ш
		"ш": &Command{
			Name:    "ш",
			Cd:      25,
			Level:   MIDDLE,
			Handler: bot.Markov,
		},
	}
}

func (cmd *Command) ExecCommand(level int) error {
	if level >= cmd.Level {
		err := cmd.Handler(cmd.Params)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("!" + cmd.Name + ": Not enough rights")
}

func (bot *Bot) Cooldown(command string, level int) error {
	if level >= TOP {
		return nil
	}
	t := time.Since(bot.Commands[command].LastUsage)
	if t >= time.Duration(bot.Commands[command].Cd)*time.Second {
		bot.Commands[command].LastUsage = time.Now()
		return nil
	} else {
		return errors.New("Command on cooldown")
	}
}

func (bot *Bot) StopVoteCommand(msg *Message) error {
	bot.Status = "Running"
	return nil
}

func parseLogTime(start, end string) (time.Time, time.Time, error) {
	layout := "2006-01-02 15:04:05 -0700 MST"
	timeStart, err := time.Parse(layout, strings.TrimSpace(start)+":00 +0300 MSK")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	timeEnd, err := time.Parse(layout, strings.TrimSpace(end)+":00 +0300 MSK")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return timeStart, timeEnd, nil
}

func logsParse(str, username string, timeStart, timeEnd time.Time) (string, error) {
	layout := "2006-01-02 15:04:05 -0700 MST"
	var tt string
	if strings.Contains(str, "[") {
		tt = strings.Split(strings.Split(str, "[")[1], "]")[0]
		timeq, err := time.Parse(layout, tt)
		if err != nil {
			return "", err
		}
		if timeq.Before(timeEnd) && timeq.After(timeStart) {
			if strings.Count(str, ":") > 2 {
				if strings.Contains(strings.Split(str, ":")[2], username) || username == "all" {
					return str, nil
				}
			}
		}
	}
	return "", errors.New("Nothing was found")
}

func (bot *Bot) LogsCommand(msg *Message) error {
	if msg == nil {
		return errors.New("!logs: not enough params")
	}
	_, params := msg.extractCommand()
	// username, timeStart, timeEnd (utt)
	utt := strings.Split(params, ",")
	if len(utt) < 3 {
		return errors.New("!logs: wrong amount of params")
	}
	username := utt[0]
	timeStart, timeEnd, err := parseLogTime(utt[1], utt[2])
	if err != nil {
		return err
	}
	if _, err = bot.File.Seek(0, io.SeekStart); err != nil {
		panic(err)
	}
	fmt.Println(username, timeStart, timeEnd)
	r := bufio.NewScanner(bot.File)
	for r.Scan() {
		str := r.Text()
		parsedStr, err := logsParse(str, username, timeStart, timeEnd)
		if err != nil {
			continue
		}
		bot.SendMessage(parsedStr)
		if err := r.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (bot *Bot) SmartVoteCommand(msg *Message) error {
	if msg == nil {
		return errors.New("!smartvote: not enough params")
	}
	_, params := msg.extractCommand()
	bot.Utils.SmartVote.Options = make(map[string]int)
	bot.Utils.SmartVote.Votes = make(map[string]string)
	bot.Status = "smartvote"
	split := strings.Split(params, "-")
	if len(split) < 2 {
		return errors.New("!smartvote: not enough args")
	}
	str := "GOLOSOVANIE"
	bot.SendMessage(str)
	lowerBound, err := strconv.Atoi(split[0])
	if err != nil {
		return err
	}
	upperBound, err := strconv.Atoi(split[1])
	if err != nil {
		return err
	}
	for i := lowerBound; i <= upperBound; i++ {
		voteStr := strconv.Itoa(i)
		bot.Utils.SmartVote.Options[voteStr] = 0
	}
	return nil
}

func (bot *Bot) VoteOptionsCommand(msg *Message) error {
	if bot.Status != "smartvote" {
		return errors.New("There is not any vote")
	}
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
	fmt.Println(total)
	str = fmt.Sprintf("Total votes %d: ", total)
	percent := float32(bot.Utils.SmartVote.Options[keys[0]]) / float32(total) * 100
	str += fmt.Sprintf("%s: %.1f%%(%d)", keys[0], percent, bot.Utils.SmartVote.Options[keys[0]])
	for i := 1; i < length; i++ {
		percent = float32(bot.Utils.SmartVote.Options[keys[i]]) / float32(total) * 100
		str += fmt.Sprintf(", %s: %.1f%%(%d)", keys[i], percent, bot.Utils.SmartVote.Options[keys[i]])
	}
	bot.SendMessage(str)
	return nil
}

// check if there is an emote in the database
func FfzBttv(emote string) (string, error) {
	db := connectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
	}
	defer tx.Rollback()

	var str string
	if err := tx.QueryRow("SELECT url FROM ffzbttv WHERE code=$1;", emote).Scan(&str); err != nil {
		return "", err
	}

	tx.Commit()
	return str, nil
}

func emoteCache(reverse bool, url string, width int, rewrite bool, thMult float32, emote string) (string, error) {
	db := connectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	_, err = tx.Exec("CREATE TABLE IF NOT EXISTS emoteCache(emote TEXT NOT NULL, image TEXT NOT NULL DEFAULT '', reversedImage TEXT NOT NULL DEFAULT '', UNIQUE (emote) ON CONFLICT REPLACE);")
	if err != nil {
		return "", err
	}
	var str string
	column := "image"
	if reverse {
		column = "reversedImage"
	}
	if err := tx.QueryRow("SELECT "+column+" FROM emoteCache WHERE emote=$l;", emote).Scan(&str); err == sql.ErrNoRows || rewrite || str == "" {
		str, err = asciifyRequest(url, width, reverse, thMult)
		if err != nil {
			return "", err
		}
		_, err = tx.Exec("INSERT INTO emoteCache(emote, "+column+") VALUES($1,$2);", emote, str)
		if err != nil {
			return "", err
		}
	}
	tx.Commit()
	return str, nil
}

func addEmote(url, code string) error {
	db := connectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", url, code)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (bot *Bot) Asciify(msg *Message) error {
	if msg == nil {
		return errors.New("!asciify: not enough params")
	}
	cmd, body := msg.extractCommand()
	params := strings.Split(body, " ")
	level := bot.Authority[msg.Username]
	width := 30
	rewrite := false
	var thMult float32 = 1.0
	var err error
	if len(params) > 1 {
		if level < TOP {
			return errors.New("!asciify: Not enough rights to change settings")
		}
		width, err = strconv.Atoi(params[1])
		if err != nil {
			return err
		}
		if len(params) == 3 {
			thMultTemp, err := strconv.ParseFloat(params[2], 32)
			if err != nil {
				return err
			}
			thMult = float32(thMultTemp)
		}
		rewrite = true
	}

	if len(params) == 0 {
		err := errors.New("!asciify: need emote")
		return err
	}
	emote := params[0] //msg.Emotes[:strigns.Index(msg.Emotes, ":")]
	url, err := FfzBttv(emote)
	if err != nil {
		if len(msg.Emotes) > 0 {
			url = "https://static-cdn.jtvnw.net/emoticons/v1/" + msg.Emotes[:strings.Index(msg.Emotes, ":")] + "/3.0"
			addEmote(url, emote)
		} else {
			url = checkForUrl(params[0])
			if url == "" {
				return err
			}
		}
	}
	reverse := false
	if cmd == "~asciify" {
		reverse = true
	}
	asciifiedImage, err := emoteCache(reverse, url, width, rewrite, thMult, emote)
	if err != nil {
		return err
	}
	bot.SendMessage(asciifiedImage)
	return nil
}

func (bot *Bot) Markov(msg *Message) error {
	markovMsg, err := Markov(bot.Channel)
	if err != nil {
		return err
	}
	bot.SendMessage("@" + msg.Username + " " + markovMsg)
	return nil
}
