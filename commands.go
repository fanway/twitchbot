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
	Name      string
	Params    []string
	LastUsage time.Time
	Cd        int
	Level     int
	Handler   func([]string) error
}

func checkForUrl(url string) string {
	if strings.HasPrefix(url, "https://") &&
		(strings.HasSuffix(url, ".jpeg") || strings.HasSuffix(url, ".jpg") || strings.HasSuffix(url, ".png")) {
		return url
	}
	return ""
}

func (bot *Bot) ParseCommand(message, emotes, username string, level int) (*Command, error) {
	args := strings.Split(message, " ")
	if args[0] == "!" {
		return nil, errors.New("could not parse command")
	}
	if cmd, ok := bot.Commands[args[0][1:]]; ok {
		if len(args) > 1 {
			cmd.Params = args[1:]
		}
		if cmd.Name == "asciify" || cmd.Name == "asciify~" {
			width := ""
			thMult := ""
			if len(cmd.Params) > 1 {
				if level < TOP {
					return nil, errors.New("!asciify: Not enough rights to change settings")
				}
				width = cmd.Params[1]
				if len(cmd.Params) == 3 {
					thMult = cmd.Params[2]
				}
			}

			if cmd.Params == nil {
				err := errors.New("!asciify: need emote")
				return nil, err
			}
			url := checkForUrl(cmd.Params[0])
			// cmd.Params = {bool: reverse image, string: "id of an emote:code of an emote", string: "from twitch or ffzbttv", int: width, float: threshold multiplier"
			if len(emotes) > 0 {
				cmd.Params = []string{"false", strings.Split(emotes, ":")[0] + ";" + cmd.Params[0], "twitch", width, thMult}
			} else {
				cmd.Params = []string{"false", url + ";" + cmd.Params[0], "ffzbttv", width, thMult}
			}
			if cmd.Name == "asciify~" {
				cmd.Params[0] = "true"
			}
		}

		cmd.Params = append(cmd.Params, username)

		err := bot.Cooldown(cmd.Name, level)
		if err != nil {
			return nil, err
		}
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

func (bot *Bot) StopVoteCommand(params []string) error {
	bot.Status = "Running"
	return nil
}

func (bot *Bot) LogsCommand(params []string) error {
	if params == nil {
		return errors.New("!logs: not enough params")
	}
	// username, timeStart, timeEnd (utt)
	utt := strings.Split(params[0], ",")
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

func (bot *Bot) SmartVoteCommand(params []string) error {
	if params == nil {
		return errors.New("!smartvote: not enough params")
	}
	bot.Utils.SmartVote.Options = make(map[string]int)
	bot.Utils.SmartVote.Votes = make(map[string]string)
	bot.Status = "smartvote"
	split := strings.Split(params[0], "-")
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

func (bot *Bot) VoteOptionsCommand(params []string) error {
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

	tx.Commit()
	return str, nil
}

func addEmote(url, code string) error {
	db := ConnectDb()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", url, code)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (bot *Bot) Asciify(params []string) error {
	var url string
	var emote string
	var err error
	split := strings.Split(params[1], ";")
	emote = split[1]
	if split[0] == "" {
		url, err = FfzBttv(emote)
	} else {
		url = split[0]
	}
	switch params[2] {
	case "twitch":
		url = "https://static-cdn.jtvnw.net/emoticons/v1/" + split[0] + "/3.0"
		addEmote(url, emote)
	case "ffzbttv":
		if err != nil {
			return err
		}
	}
	width := 30
	var thMult float32
	thMult = 1.0
	rewrite := false
	reverse, err := strconv.ParseBool(params[0])
	if err != nil {
		return err
	}
	// width of an image parameter
	if params[3] != "" {
		width, err = strconv.Atoi(params[3])
		if err != nil {
			return err
		}
		rewrite = true
	}
	// threshold multiplier parameter
	if params[4] != "" {
		thMultTemp, err := strconv.ParseFloat(params[4], 32)
		if err != nil {
			return err
		}
		thMult = float32(thMultTemp)
		rewrite = true
	}
	asciifiedImage, err := EmoteCache(reverse, url, width, rewrite, thMult, emote)
	if err != nil {
		return err
	}
	bot.SendMessage(asciifiedImage)
	return nil
}

func (bot *Bot) Markov(params []string) error {
	msg, err := Markov(bot.Channel)
	if err != nil {
		return err
	}
	bot.SendMessage("@" + params[0] + " " + msg)
	return nil
}
