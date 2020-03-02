package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Command struct {
	Name   string
	Params string
}

func (cmd *Command) Parse(message string) error {
	args := strings.Split(message, " ")
	if args[0] == "!" {
		return errors.New("could not parse command")
	}
	cmd.Name = args[0][1:]
	if len(args) > 1 {
		cmd.Params = args[1]
	}
	return nil
}

func (bot *Bot) LogsCommand(params string) error {
	// username, timeStart, timeEnd (utt)
	utt := strings.Split(params, ",")
	if len(utt) < 3 {
		return errors.New("!logs: wrong amount of params")
	}
	layout := "2006-01-02 15:04:05 -0700 MST"
	username := utt[0]
	timeStart, err := time.Parse(layout, "2019-"+strings.TrimSpace(utt[1])+":00 +0300 MSK")
	if err != nil {
		return err
	}
	timeEnd, err := time.Parse(layout, "2019-"+strings.TrimSpace(utt[2])+":00 +0300 MSK")
	if err != nil {
		return err
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
				return err
			}
			if timeq.Before(timeEnd) && timeq.After(timeStart) {
				if strings.Count(str, ":") > 2 {
					if strings.Contains(strings.Split(str, ":")[2], username) || username == "all" {
						bot.SendMessage(str)
					}
				}
			}
		}
		if err := r.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (bot *Bot) SmartVoteCommand(params string) error {
	bot.Utils.SmartVote.Options = make(map[string]int)
	bot.Utils.SmartVote.Votes = make(map[string]string)
	bot.Status = "smartvote"
	split := strings.Split(params, ",")
	if len(split) < 2 {
		return errors.New("!smartvote: not enough args")
	}
	str := "GOLOSOVANIE"
	bot.SendMessage(str)
	r := strings.Split(split[1], "-")
	lowerBound, err := strconv.Atoi(r[0])
	if err != nil {
		return err
	}
	upperBound, err := strconv.Atoi(r[1])
	if err != nil {
		return err
	}
	for i := lowerBound; i <= upperBound; i++ {
		voteStr := strconv.Itoa(i)
		bot.Utils.SmartVote.Options[voteStr] = 0
	}
	return nil
}

func (bot *Bot) VoteOptionsCommand() error {
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
	bot.SendMessage(str)
	return nil
}

func FfzBttv(emote string) (string, error) {
	db := ConnectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
	}
	defer tx.Rollback()

	var str string
	if err := tx.QueryRow("SELECT url FROM ffzbttv WHERE code=$1;", emote).Scan(&str); err != nil {
		return "", err
	}

	return str, nil

}

func (bot *Bot) Asciify(emote string, state string) error {
	var url string
	var err error
	switch state {
	case "twitch":
		url = "https://static-cdn.jtvnw.net/emoticons/v1/" + emote + "/3.0"
	case "ffzbttv":
		url, err = FfzBttv(emote)
		if err != nil {
			return err
		}
	}
	bot.SendMessage(EmoteCache(url))
	return nil
}
