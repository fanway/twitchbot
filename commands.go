package main

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
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
	if splitIndex == -1 {
		splitIndex = len(message.Text)
	}
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
		// !r <song name>
		"r": &Command{
			Name:    "r",
			Cd:      0,
			Level:   LOW,
			Handler: bot.RequestTrack,
		},
		// !song
		"song": &Command{
			Name:    "song",
			Cd:      10,
			Level:   LOW,
			Handler: bot.CurrentTrack,
		},
		"mr": &Command{
			Name:    "mr",
			Cd:      10,
			Level:   LOW,
			Handler: bot.GetUserSongs,
		},
		"commands": &Command{
			Name:    "commands",
			Cd:      3,
			Level:   LOW,
			Handler: bot.GetCommands,
		},
	}
}

func (cmd *Command) ExecCommand(level int) error {
	err := cmd.Cooldown(level)
	if err != nil {
		log.Println(err)
		return err
	}
	if level >= cmd.Level {
		err := cmd.Handler(cmd.Params)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("!" + cmd.Name + ": Not enough rights")
}

func (cmd *Command) Cooldown(level int) error {
	if level >= TOP {
		return nil
	}
	t := time.Since(cmd.LastUsage)
	if t >= time.Duration(cmd.Cd)*time.Second {
		cmd.LastUsage = time.Now()
		return nil
	} else {
		return errors.New("Command on cooldown")
	}
}

func (bot *Bot) StopVoteCommand(msg *Message) error {
	bot.Status = "Running"
	return nil
}

// Delete all songs up to the current one
func (req *RequestedSongs) clear() (int, error) {
	track, err := getCurrentTrack()
	if err != nil {
		log.Println(err)
		return 0, err
	}
	req.Lock()
	defer req.Unlock()
	var i int
	for i = 0; i < len(req.Songs); i++ {
		if req.Songs[i].Song == track {
			break
		}
	}
	// How many previous songs we want to keep
	const keep = 1
	if i >= keep && i < len(req.Songs) {
		req.Songs = req.Songs[i-keep:]
	}
	return i, nil
}

// Get all songs that the user requested
func (bot *Bot) GetUserSongs(msg *Message) error {
	songs := []string{}
	index, err := bot.Utils.RequestedSongs.clear()
	if err != nil {
		return err
	}
	bot.Utils.RequestedSongs.RLock()
	defer bot.Utils.RequestedSongs.RUnlock()
	var accDuration time.Duration
	for i := index; i < len(bot.Utils.RequestedSongs.Songs); i++ {
		if bot.Utils.RequestedSongs.Songs[i].Username == msg.Username {
			var t string
			if i == index {
				t = "playing"
			} else {
				t = accDuration.Round(time.Second).String()
			}
			songs = append(songs, bot.Utils.RequestedSongs.Songs[i].Song+" ["+t+"]")
		}
		accDuration += bot.Utils.RequestedSongs.Songs[i].Duration
	}

	if len(songs) == 0 {
		return errors.New("No songs found")
	}

	retMsg := " You are requested: " + songs[0]
	for i := 1; i < len(songs); i++ {
		retMsg += ", " + songs[i]
	}
	bot.SendMessage("@" + msg.Username + retMsg)
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

func logsParse(str, msg, username string, timeStart, timeEnd time.Time) (string, error) {
	layout := "2006-01-02 15:04:05 -0700 MST"
	re := regexp.MustCompile(`\[(.*?)\] (.*?): (.*)`)
	match := re.FindStringSubmatch(str)
	timeq, err := time.Parse(layout, match[1])
	if err != nil {
		return "", err
	}
	if timeq.Before(timeEnd) && timeq.After(timeStart) {
		if msg != "" {
			if strings.Contains(match[3], msg) {
				return match[2], nil
			}
		} else if username == match[2] || username == "all" {
			return str, nil
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
		parsedStr, err := logsParse(str, "", username, timeStart, timeEnd)
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
	bot.Status = "Smartvote"
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
	if bot.Status != "Smartvote" {
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

func (bot *Bot) RequestTrack(msg *Message) error {
	_, params := msg.extractCommand()
	track, err := searchTrack(params)
	if err != nil {
		log.Println(err)
		return err
	}
	if len(track.Tracks.Items) == 0 {
		bot.SendMessage("@" + msg.Username + " track wasn't found")
		return errors.New("Track wasn't found")
	}

	err = addToPlaylist(track.Tracks.Items[0].URI)
	if err != nil {
		log.Println(err)
		return err
	}
	trackName := track.Tracks.Items[0].Artists[0].Name + " - " + track.Tracks.Items[0].Name
	bot.Utils.RequestedSongs.Lock()
	defer bot.Utils.RequestedSongs.Unlock()
	bot.Utils.RequestedSongs.Songs = append(bot.Utils.RequestedSongs.Songs, Song{Username: msg.Username, Song: trackName, Duration: time.Duration(track.Tracks.Items[0].DurationMs) * time.Millisecond})

	bot.SendMessage("@" + msg.Username + " " + trackName + " was added a playlist")
	return nil
}

func (bot *Bot) CurrentTrack(msg *Message) error {
	track, err := getCurrentTrack()
	if err != nil {
		log.Println(err)
		return err
	}
	if track != "" {
		bot.SendMessage("@" + msg.Username + " " + track)
	}
	return nil
}

func (bot *Bot) GetCommands(msg *Message) error {
	if len(bot.Commands) == 0 {
		return errors.New("No commands found")
	}
	var keys []string
	for k := range bot.Commands {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	message := keys[0]
	for i := 1; i < len(keys); i++ {
		message += ", " + keys[i]
	}
	return nil
}
