package main

import (
	"bufio"
	"database/sql"
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
	"time"
	pb "twitchStats/commands/pb"
	"twitchStats/database"
	"twitchStats/logsparser"
	"twitchStats/markov"
	"twitchStats/request"
	"twitchStats/spotify"

	"google.golang.org/grpc"
)

const (
	LOW = iota
	MIDDLE
	TOP
)

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
		// !logs username, timeStart, timeEnd
		"logs": &Command{
			Name:    "logs",
			Cd:      60,
			Level:   TOP,
			Handler: s.LogsCommand,
		},
		// !smartvote lowerBound, upperBound
		"smartvote": &Command{
			Name:    "smartvote",
			Cd:      30,
			Level:   TOP,
			Handler: s.SmartVoteCommand,
		},
		// !stopvote
		"stopvote": &Command{
			Name:    "stopvote",
			Cd:      15,
			Level:   TOP,
			Handler: s.StopVoteCommand,
		},
		// !voteoptions
		"voteoptions": &Command{
			Name:    "voteoptions",
			Cd:      5,
			Level:   MIDDLE,
			Handler: s.VoteOptionsCommand,
		},
		// !asciify <emote>
		"asciify": &Command{
			Name:    "asciify",
			Cd:      10,
			Level:   MIDDLE,
			Handler: s.Asciify,
		},
		// !asciify <emote>
		"asciify~": &Command{
			Name:    "asciify~",
			Cd:      10,
			Level:   MIDDLE,
			Handler: s.Asciify,
		},
		// !ш
		"ш": &Command{
			Name:    "ш",
			Cd:      25,
			Level:   MIDDLE,
			Handler: s.Markov,
		},
		// !r <song name>
		"r": &Command{
			Name:    "r",
			Cd:      0,
			Level:   LOW,
			Handler: s.RequestTrack,
		},
		// !song
		"song": &Command{
			Name:    "song",
			Cd:      10,
			Level:   LOW,
			Handler: s.CurrentTrack,
		},
		"mr": &Command{
			Name:    "mr",
			Cd:      10,
			Level:   LOW,
			Handler: s.GetUserSongs,
		},
		"commands": &Command{
			Name:    "commands",
			Cd:      3,
			Level:   LOW,
			Handler: s.GetCommands,
		},
		"level": &Command{
			Name:    "level",
			Cd:      10,
			Level:   LOW,
			Handler: s.GetLevel,
		},
		"vote": &Command{
			Name:    "vote",
			Cd:      0,
			Level:   LOW,
			Handler: s.VoteCommand,
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
	Options map[string]int
	Votes   map[string]string
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
		err := cmd.Cooldown(level)
		if err != nil {
			log.Println(err)
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
	stream.Send(&pb.ReturnMessage{Text: "", Status: msg.Status})
	return nil
}

// Delete all songs up to the current one
func (req *RequestedSongs) clear() (int, error) {
	track, err := spotify.GetCurrentTrack()
	if err != nil {
		log.Println(err)
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

	retMsg := " Your requested songs: " + songs[0]
	for i := 1; i < len(songs); i++ {
		retMsg += ", " + songs[i]
	}
	stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + retMsg, Status: msg.Status})
	return nil
}

func (s *CommandsServer) RequestTrack(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, params := extractCommand(msg)
	track, err := spotify.SearchTrack(params)
	if err != nil {
		log.Println(err)
		return err
	}
	if len(track.Tracks.Items) == 0 {

		stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + " track wasn't found", Status: msg.Status})
		return errors.New("Track wasn't found")
	}

	err = spotify.AddToPlaylist(track.Tracks.Items[0].URI)
	if err != nil {
		log.Println(err)
		return err
	}
	trackName := track.Tracks.Items[0].Artists[0].Name + " - " + track.Tracks.Items[0].Name
	s.m[msg.Username].Utils.RequestedSongs.Lock()
	defer s.m[msg.Username].Utils.RequestedSongs.Unlock()
	s.m[msg.Username].Utils.RequestedSongs.Songs = append(s.m[msg.Username].Utils.RequestedSongs.Songs, Song{Username: msg.Username, SongName: trackName, Uri: track.Tracks.Items[0].URI, Duration: time.Duration(track.Tracks.Items[0].DurationMs) * time.Millisecond})
	stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + " " + trackName + " was added to the playlist", Status: msg.Status})
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
		log.Println(err)
		return err
	}
	if track != "" {
		stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + " " + track, Status: msg.Status})
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
		stream.Send(&pb.ReturnMessage{Text: parsedStr, Status: msg.Status})
		if err := r.Err(); err != nil {
			return err
		}
	}
	return nil
}

func (s *CommandsServer) SmartVoteCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	_, params := extractCommand(msg)
	s.m[msg.Channel].Utils.SmartVote.Options = make(map[string]int)
	s.m[msg.Channel].Utils.SmartVote.Votes = make(map[string]string)
	split := strings.Split(params, "-")
	if len(split) < 2 {
		return errors.New("!smartvote: not enough args")
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
	for i := lowerBound; i <= upperBound; i++ {
		voteStr := strconv.Itoa(i)
		s.m[msg.Channel].Utils.SmartVote.Options[voteStr] = 0
	}
	stream.Send(&pb.ReturnMessage{Text: str, Status: "SmartVote"})
	return nil
}

func (s *CommandsServer) VoteOptionsCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	if msg.Status != "Smartvote" {
		return errors.New("There is not any vote")
	}
	length := len(s.m[msg.Channel].Utils.SmartVote.Options)
	keys := make([]string, length)
	i := 0
	for k := range s.m[msg.Channel].Utils.SmartVote.Options {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	var str string
	total := len(s.m[msg.Channel].Utils.SmartVote.Votes)
	fmt.Println(total)
	str = fmt.Sprintf("Total votes %d: ", total)
	percent := float32(s.m[msg.Channel].Utils.SmartVote.Options[keys[0]]) / float32(total) * 100
	str += fmt.Sprintf("%s: %.1f%%(%d)", keys[0], percent, s.m[msg.Channel].Utils.SmartVote.Options[keys[0]])
	for i := 1; i < length; i++ {
		percent = float32(s.m[msg.Channel].Utils.SmartVote.Options[keys[i]]) / float32(total) * 100
		str += fmt.Sprintf(", %s: %.1f%%(%d)", keys[i], percent, s.m[msg.Channel].Utils.SmartVote.Options[keys[i]])
	}
	stream.Send(&pb.ReturnMessage{Text: str, Status: msg.Status})
	return nil
}

func (s *CommandsServer) VoteCommand(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	if msg.Status != "Smartvote" {
		return errors.New("Wrong status")
	}
	_, body := extractCommand(msg)
	if _, ok := s.m[msg.Channel].Utils.SmartVote.Options[body]; ok {
		// consider only the first vote
		if _, ok := s.m[msg.Channel].Utils.SmartVote.Votes[msg.Username]; !ok {
			s.m[msg.Channel].Utils.SmartVote.Options[body]++
			s.m[msg.Channel].Utils.SmartVote.Votes[msg.Username] = body
		}
	}
	return nil
}

// check if there is an emote in the database
func FfzBttv(emote string) (string, error) {
	db := database.Connect()
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
	stream.Send(&pb.ReturnMessage{Text: asciifiedImage, Status: msg.Status})
	return nil
}

func (s *CommandsServer) Markov(msg *pb.Message, stream pb.Commands_ParseAndExecServer) error {
	markovMsg, err := markov.Markov(msg.Channel)
	if err != nil {
		return err
	}
	stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + " " + markovMsg, Status: msg.Status})
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
	stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + " " + commands, Status: msg.Status})
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
	stream.Send(&pb.ReturnMessage{Text: "@" + msg.Username + " " + "Your level is: " + message, Status: msg.Status})
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
	fmt.Println("Grpc server started")
	grpcServer.Serve(lis)
}
