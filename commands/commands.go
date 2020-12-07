package main

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	pb "twitchStats/commands/pb"
	"twitchStats/database"
	"twitchStats/database/cache"
	"twitchStats/logsparser"
	"twitchStats/markov"
	"twitchStats/request"
	"twitchStats/spotify"
	"twitchStats/terminal"

	"github.com/gomodule/redigo/redis"
	"google.golang.org/grpc"
)

const (
	LOW = iota
	MIDDLE
	TOP
)

var pool *redis.Pool

type CommandsServer struct {
	pb.UnimplementedCommandsServer

	m map[string]*Commands
}

type Commands struct {
	Utils    Utils
	Commands map[string]*Command
}

func (s *CommandsServer) initCommands(channel string) {
	logfile, err := os.OpenFile("../"+channel+".log", os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatal(err)
	}
	c := &Commands{Utils: Utils{File: logfile}, Commands: map[string]*Command{
		// !logs <username, timeStart, timeEnd>
		"logs": &Command{
			Enabled: true,
			Name:    "logs",
			Cd:      60,
			Level:   TOP,
			Handler: s.LogsCommand,
		},
		// !smartvote <lowerBound, upperBound>
		"smartvote": &Command{
			Enabled: true,
			Name:    "smartvote",
			Cd:      30,
			Level:   TOP,
			Handler: s.SmartVoteCommand,
		},
		// !stopvote
		"stopvote": &Command{
			Enabled: true,
			Name:    "stopvote",
			Cd:      15,
			Level:   TOP,
			Handler: s.StopVoteCommand,
		},
		// !voteoptions
		"voteoptions": &Command{
			Enabled: true,
			Name:    "voteoptions",
			Cd:      5,
			Level:   MIDDLE,
			Handler: s.VoteOptionsCommand,
		},
		// !asciify <emote>
		"asciify": &Command{
			Enabled: true,
			Name:    "asciify",
			Cd:      10,
			Level:   MIDDLE,
			Handler: s.Asciify,
		},
		// !asciify <emote>
		"asciify~": &Command{
			Enabled: true,
			Name:    "asciify~",
			Cd:      10,
			Level:   MIDDLE,
			Handler: s.Asciify,
		},
		// !ш
		"ш": &Command{
			Enabled: true,
			Name:    "ш",
			Cd:      25,
			Level:   MIDDLE,
			Handler: s.Markov,
		},
		// !r <song name>
		"r": &Command{
			Enabled: true,
			Name:    "r",
			Cd:      0,
			Level:   LOW,
			Handler: s.RequestTrack,
		},
		// !song
		"song": &Command{
			Enabled: true,
			Name:    "song",
			Cd:      10,
			Level:   LOW,
			Handler: s.CurrentTrack,
		},
		// !mr
		"mr": &Command{
			Enabled: true,
			Name:    "mr",
			Cd:      10,
			Level:   LOW,
			Handler: s.GetUserSongs,
		},
		// !commands
		"commands": &Command{
			Enabled: true,
			Name:    "commands",
			Cd:      3,
			Level:   LOW,
			Handler: s.GetCommands,
		},
		// !level
		"level": &Command{
			Enabled: true,
			Name:    "level",
			Cd:      10,
			Level:   LOW,
			Handler: s.GetLevel,
		},
		// !vote <option>
		"vote": &Command{
			Enabled: true,
			Name:    "vote",
			Cd:      0,
			Level:   LOW,
			Handler: s.VoteCommand,
		},
		// !remind <time> <message>
		"remind": &Command{
			Enabled: true,
			Name:    "remind",
			Cd:      5,
			Level:   MIDDLE,
			Handler: s.RemindCommand,
		},
		// !afk <message>
		"afk": &Command{
			Enabled: true,
			Name:    "afk",
			Cd:      5,
			Level:   MIDDLE,
			Handler: s.AfkCommand,
		},
		// !stalk <username>
		"stalk": &Command{
			Enabled: true,
			Name:    "stalk",
			Cd:      5,
			Level:   MIDDLE,
			Handler: s.StalkCommand,
		},
		// !disable <command>
		"disable": &Command{
			Enabled: true,
			Name:    "disable",
			Cd:      5,
			Level:   TOP,
			Handler: s.DisableCommand,
		},
		// !enable <command>
		"enable": &Command{
			Enabled: true,
			Name:    "enable",
			Cd:      5,
			Level:   TOP,
			Handler: s.EnableCommand,
		},
	}}
	s.m[channel] = c
}

type Utils struct {
	SmartVote      SmartVote
	RequestedSongs RequestedSongs
	File           *os.File
}

type SmartVote struct {
	sync.Mutex
	Options []*int32
	Votes   map[string]int
}

type Song struct {
	Username string
	SongName string
	Uri      string
	Duration time.Duration
}

type RequestedSongs struct {
	sync.RWMutex
	Songs []Song
}

func extractCommand(msg *pb.Message) (string, string) {
	index := strings.Index(msg.Text, " ")
	if index == -1 {
		return msg.Text[1:], ""
	}
	return msg.Text[1:index], msg.Text[index+1:]
}

func (s *CommandsServer) ParseAndExec(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	if _, ok := s.m[msg.Channel]; !ok {
		s.initCommands(msg.Channel)
	}
	splitIndex := strings.Index(msg.Text, " ")
	level := int(msg.Level)
	if splitIndex == -1 {
		splitIndex = len(msg.Text)
	}
	if cmd, ok := s.m[msg.Channel].Commands[msg.Text[1:splitIndex]]; ok {
		if !cmd.Enabled {
			return errors.New(cmd.Name + " command is disabled")
		}
		err := cmd.Cooldown(level)
		if err != nil {
			terminal.Output.Log(err)
			return err
		}
		if level >= cmd.Level {
			err := cmd.Handler(msg, stream)
			if err != nil {
				return err
			}
			return nil
		}
		return errors.New("!" + cmd.Name + ": Not enough rights")
	}
	return errors.New("could not parse command")
}

type Command struct {
	Enabled   bool
	Name      string
	LastUsage time.Time
	Cd        int
	Level     int
	Handler   func(*pb.Message, pb.Commands_ParseAndExecServer) error
}

func checkForUrl(url string) string {
	if strings.HasPrefix(url, "https://") &&
		(strings.HasSuffix(url, ".jpeg") || strings.HasSuffix(url, ".jpg") || strings.HasSuffix(url, ".png")) {
		return url
	}
	return ""
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

func (s *CommandsServer) StopVoteCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	stream.Send(&pb.ReturnMessage{Text: ""})
	return nil
}

// Delete all songs up to the current one
func (req *RequestedSongs) clear() (int, error) {
	track, err := spotify.GetCurrentTrack()
	if err != nil {
		terminal.Output.Log(err)
		return 0, err
	}
	req.Lock()
	defer req.Unlock()
	var i int
	reqLength := len(req.Songs)
	for i = 0; i < reqLength; i++ {
		if req.Songs[i].SongName == track {
			break
		}
	}
	if i == reqLength {
		return 0, errors.New("No songs found")
	}
	// How many previous songs we want to keep
	const keep = 1
	if i >= keep && i < len(req.Songs) {
		req.Songs = req.Songs[i-keep:]
	}
	return keep, nil
}

// Get all songs that the user requested
func (s *CommandsServer) GetUserSongs(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	songs := []string{}
	index, err := s.m[msg.Username].Utils.RequestedSongs.clear()
	if err != nil {
		return err
	}
	s.m[msg.Username].Utils.RequestedSongs.RLock()
	defer s.m[msg.Username].Utils.RequestedSongs.RUnlock()
	var accDuration time.Duration
	for i := index; i < len(s.m[msg.Username].Utils.RequestedSongs.Songs); i++ {
		if s.m[msg.Username].Utils.RequestedSongs.Songs[i].Username == msg.Username {
			var t string
			if i == index {
				t = "playing"
			} else {
				t = accDuration.Round(time.Second).String()
			}
			songs = append(songs, s.m[msg.Username].Utils.RequestedSongs.Songs[i].SongName+" ["+t+"]")
		}
		accDuration += s.m[msg.Username].Utils.RequestedSongs.Songs[i].Duration
	}

	if len(songs) == 0 {
		return errors.New("No songs found")
	}

	retMsg := "Your requested songs: " + songs[0]
	for i := 1; i < len(songs); i++ {
		retMsg += ", " + songs[i]
	}
	stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s %s", msg.Username, retMsg)})
	return nil
}

func (s *CommandsServer) RequestTrack(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, params := extractCommand(msg)
	track, err := spotify.SearchTrack(params)
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	if len(track.Tracks.Items) == 0 {

		stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s track wansn't found", msg.Username)})
		return errors.New("Track wasn't found")
	}

	err = spotify.AddToPlaylist(track.Tracks.Items[0].URI)
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	trackName := track.Tracks.Items[0].Artists[0].Name + " - " + track.Tracks.Items[0].Name
	s.m[msg.Username].Utils.RequestedSongs.Lock()
	defer s.m[msg.Username].Utils.RequestedSongs.Unlock()
	s.m[msg.Username].Utils.RequestedSongs.Songs = append(s.m[msg.Username].Utils.RequestedSongs.Songs, Song{Username: msg.Username, SongName: trackName, Uri: track.Tracks.Items[0].URI, Duration: time.Duration(track.Tracks.Items[0].DurationMs) * time.Millisecond})
	stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s %s was added to the playlist", msg.Username, trackName)})
	return nil
}

func (s *CommandsServer) RemoveRequestedTrack(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, params := extractCommand(msg)
	index, err := s.m[msg.Username].Utils.RequestedSongs.clear()
	if err != nil {
		return err
	}
	err = spotify.RemoveTrack(s.m[msg.Username].Utils.RequestedSongs.Songs[index].Uri)
	if err != nil {
		return nil
	}
	s.m[msg.Username].Utils.RequestedSongs.Lock()
	defer s.m[msg.Username].Utils.RequestedSongs.Unlock()
	i := len(s.m[msg.Username].Utils.RequestedSongs.Songs)
	for ; i >= 0; i-- {
		if params == s.m[msg.Username].Utils.RequestedSongs.Songs[i].SongName {
			break
		}
	}
	if i == -1 {
		return errors.New("No track found")
	}
	s.m[msg.Username].Utils.RequestedSongs.Songs = append(s.m[msg.Username].Utils.RequestedSongs.Songs[:i], s.m[msg.Username].Utils.RequestedSongs.Songs[i+1:]...)
	return nil
}

func (s *CommandsServer) CurrentTrack(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	track, err := spotify.GetCurrentTrack()
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	if track != "" {
		stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s %s", msg.Username, track)})
	}
	return nil
}

func (s *CommandsServer) LogsCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, params := extractCommand(msg)
	// username, timeStart, timeEnd (utt)
	utt := strings.Split(params, ",")
	if len(utt) < 3 {
		return errors.New("!logs: wrong amount of params")
	}
	username := utt[0]
	timeStart, timeEnd, err := logsparser.ParseTime(utt[1], utt[2])
	if err != nil {
		return err
	}
	if _, err = s.m[msg.Channel].Utils.File.Seek(0, io.SeekStart); err != nil {
		panic(err)
	}
	fmt.Println(username, timeStart, timeEnd)
	r := bufio.NewScanner(s.m[msg.Channel].Utils.File)
	for r.Scan() {
		str := r.Text()
		parsedStr, err := logsparser.Parse(str, "", username, timeStart, timeEnd)
		if err != nil {
			continue
		}
		stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("[%s] %s: %s", parsedStr[1], parsedStr[2], parsedStr[3])})
	}
	if err := r.Err(); err != nil {
		return err
	}
	return nil
}

func (s *CommandsServer) SmartVoteCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, params := extractCommand(msg)
	split := strings.Split(params, "-")
	if len(split) < 2 {
		return errors.New("!smartvote: not enough args")
	}

	conn := pool.Get()
	defer conn.Close()
	_, err := conn.Do("HSET", msg.Channel, "status", "Smartvote")
	if err != nil {
		return err
	}

	str := "GOLOSOVANIE"
	lowerBound, err := strconv.Atoi(split[0])
	if err != nil {
		return err
	}
	upperBound, err := strconv.Atoi(split[1])
	if err != nil {
		return err
	}
	s.m[msg.Channel].Utils.SmartVote.Options = make([]*int32, upperBound+1)
	s.m[msg.Channel].Utils.SmartVote.Votes = make(map[string]int)
	for i := lowerBound; i <= upperBound; i++ {
		var value int32
		s.m[msg.Channel].Utils.SmartVote.Options[i] = &value
	}
	stream.Send(&pb.ReturnMessage{Text: str})
	return nil
}

func (s *CommandsServer) VoteOptionsCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	conn := pool.Get()
	defer conn.Close()
	status, err := redis.String(conn.Do("HGET", msg.Channel, "status"))
	if err != nil {
		return err
	}
	if status != "Smartvote" {
		return errors.New("There is not any vote")
	}
	keys := []int{}
	for i, v := range s.m[msg.Channel].Utils.SmartVote.Options {
		if v == nil {
			continue
		}
		keys = append(keys, i)
	}
	var str string
	total := len(s.m[msg.Channel].Utils.SmartVote.Votes)
	str = fmt.Sprintf("Total votes %d: ", total)
	value := atomic.LoadInt32(s.m[msg.Channel].Utils.SmartVote.Options[keys[0]])
	percent := float32(value) / float32(total) * 100
	str += fmt.Sprintf("%d: %.1f%%(%d)", keys[0], percent, value)
	for i := 1; i < len(keys); i++ {
		value = atomic.LoadInt32(s.m[msg.Channel].Utils.SmartVote.Options[keys[i]])
		percent = float32(value) / float32(total) * 100
		str += fmt.Sprintf(", %d: %.1f%%(%d)", keys[i], percent, value)
	}
	stream.Send(&pb.ReturnMessage{Text: str})
	return nil
}

func (s *CommandsServer) VoteCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	conn := pool.Get()
	defer conn.Close()
	status, err := redis.String(conn.Do("HGET", msg.Channel, "status"))
	if err != nil {
		return err
	}
	if status != "Smartvote" {
		return errors.New("Wrong status")
	}
	_, body := extractCommand(msg)
	vote, err := strconv.Atoi(body)
	if err != nil {
		return err
	}
	if vote < 0 || vote >= len(s.m[msg.Channel].Utils.SmartVote.Options) ||
		s.m[msg.Channel].Utils.SmartVote.Options[vote] == nil {
		return errors.New("!vote: out of bounds")
	}
	// consider only one vote
	atomic.AddInt32(s.m[msg.Channel].Utils.SmartVote.Options[vote], 1)
	s.m[msg.Channel].Utils.SmartVote.Lock()
	if v, ok := s.m[msg.Channel].Utils.SmartVote.Votes[msg.Username]; ok {
		atomic.AddInt32(s.m[msg.Channel].Utils.SmartVote.Options[v], -1)
	}
	s.m[msg.Channel].Utils.SmartVote.Votes[msg.Username] = vote
	s.m[msg.Channel].Utils.SmartVote.Unlock()
	return nil
}

// check if there is an emote in the database
func FfzBttv(emote string) (string, error) {
	db := database.Connect()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		terminal.Output.Log(err)
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
	db := database.Connect()
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
		str, err = request.Asciify(url, width, reverse, thMult)
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
	db := database.Connect()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		terminal.Output.Log(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec("INSERT INTO ffzbttv(url, code) VALUES($1,$2);", url, code)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *CommandsServer) Asciify(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	cmd, body := extractCommand(msg)
	params := strings.Split(body, " ")
	level := msg.Level
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

	if len(params[0]) == 0 {
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
	stream.Send(&pb.ReturnMessage{Text: asciifiedImage})
	return nil
}

func (s *CommandsServer) Markov(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	markovMsg, err := markov.Markov(msg.Channel)
	if err != nil {
		return err
	}
	stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s %s", msg.Username, markovMsg)})
	return nil
}

func (s *CommandsServer) GetCommands(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	if len(s.m[msg.Channel].Commands) == 0 {
		return errors.New("No commands found")
	}
	var keys []string
	for k, v := range s.m[msg.Channel].Commands {
		if int(msg.Level) >= v.Level {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	commands := keys[0]
	for i := 1; i < len(keys); i++ {
		commands += ", " + keys[i]
	}
	stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s %s", msg.Username, commands)})
	return nil
}

func (s *CommandsServer) GetLevel(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	var message string
	switch msg.Level {
	case 0:
		message = "low"
	case 1:
		message = "middle"
	case 2:
		message = "top"
	}
	stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s Your level is: %s", msg.Username, message)})
	return nil
}

func (s *CommandsServer) RemindCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, body := extractCommand(msg)
	params := strings.Split(body, " ")
	if len(params[0]) == 0 {
		return errors.New("!remind: not enough params")
	}
	t, err := time.ParseDuration(params[0])
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	var remindMessage string
	if len(params) < 2 {
		remindMessage = ""
	} else {
		remindMessage = params[1]
	}
	retMsg := "@" + msg.Username + " " + remindMessage
	time.AfterFunc(t, func() {
		conn := pool.Get()
		defer conn.Close()
		conn.Send("PUBLISH", "reminders", retMsg)
		conn.Flush()
	})
	return nil
}

func (s *CommandsServer) AfkCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, body := extractCommand(msg)
	conn := pool.Get()
	defer conn.Close()
	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(struct {
		Message string
		Time    time.Time
	}{body, time.Now()})
	if err != nil {
		terminal.Output.Log(err)
		return err
	}
	_, err = conn.Do("HSET", msg.Channel, "afk:"+msg.Username, b.Bytes())
	if err != nil {
		return err
	}
	return nil
}

func (s *CommandsServer) StalkCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, body := extractCommand(msg)
	timeNow := time.Now()
	timeStart := timeNow.Add(-24 * time.Hour)
	timeEnd := timeNow
	if _, err := s.m[msg.Channel].Utils.File.Seek(0, io.SeekStart); err != nil {
		panic(err)
	}
	r := bufio.NewScanner(s.m[msg.Channel].Utils.File)
	retMessage := "Found nothing, sorry! :)"
	for r.Scan() {
		str := r.Text()
		parsedStr, err := logsparser.Parse(str, "", body, timeStart, timeEnd)
		if err != nil {
			continue
		}
		t, err := time.Parse(logsparser.Layout, parsedStr[1])
		if err != nil {
			continue
		}
		retMessage = fmt.Sprintf("%s was seen %s ago, last message: %s", body, time.Since(t).Truncate(time.Second), parsedStr[3])
	}
	stream.Send(&pb.ReturnMessage{Text: fmt.Sprintf("@%s %s", msg.Username, retMessage)})
	if err := r.Err(); err != nil {
		return err
	}
	return nil
}

func (s *CommandsServer) DisableCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, body := extractCommand(msg)
	retMessage := "Command wasn't found"
	if _, ok := s.m[msg.Channel].Commands[body]; ok {
		s.m[msg.Channel].Commands[body].Enabled = false
		retMessage = fmt.Sprintf("!%s command has been disabled", body)
	}
	stream.Send(&pb.ReturnMessage{Text: retMessage})
	return nil
}

func (s *CommandsServer) EnableCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, body := extractCommand(msg)
	retMessage := "Command wasn't found"
	if _, ok := s.m[msg.Channel].Commands[body]; ok {
		s.m[msg.Channel].Commands[body].Enabled = true
		retMessage = fmt.Sprintf("!%s command has been enabled", body)
	}
	stream.Send(&pb.ReturnMessage{Text: retMessage})
	return nil
}

func newServer() *CommandsServer {
	s := &CommandsServer{m: make(map[string]*Commands)}
	return s
}

func main() {
	lis, err := net.Listen("tcp", "localhost:3434")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	pb.RegisterCommandsServer(grpcServer, newServer())
	pool = cache.GetPool()
	fmt.Println("Grpc server started")
	grpcServer.Serve(lis)
}
