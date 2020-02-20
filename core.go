package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	//"io/ioutil"
)

type IdData struct {
	Data []struct {
		ID              string `json:"id"`
		Login           string `json:"login"`
		DisplayName     string `json:"display_name"`
		Type            string `json:"type"`
		BroadcasterType string `json:"broadcaster_type"`
		Description     string `json:"description"`
		ProfileImageURL string `json:"profile_image_url"`
		OfflineImageURL string `json:"offline_image_url"`
		ViewCount       int    `json:"view_count"`
	} `json:"data"`
}

type Followers struct {
	Total int `json:"total"`
	Data  []struct {
		FromID     string `json:"from_id"`
		FromName   string `json:"from_name"`
		ToID       string `json:"to_id"`
		ToName     string `json:"to_name"`
		FollowedAt string `json:"followed_at"`
	} `json:"data"`
	Pagination struct {
		Cursor string `json:"cursor"`
	} `json:"pagination"`
}

type ChatData struct {
	Links struct {
	} `json:"_links"`
	ChatterCount int `json:"chatter_count"`
	Chatters     struct {
		Broadcaster []string `json:"broadcaster"`
		Vips        []string `json:"vips"`
		Moderators  []string `json:"moderators"`
		Staff       []string `json:"staff"`
		Admins      []string `json:"admins"`
		GlobalMods  []string `json:"global_mods"`
		Viewers     []string `json:"viewers"`
	} `json:"chatters"`
}

func findPerson(name string) {
	db, err := sql.Open("sqlite3", "./data.db?_busy_timeout=5000&cache=shared&mode=rwc")
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		return
	}
	defer tx.Rollback()
	client := &http.Client{}
	var id string
	row := tx.QueryRow("SELECT FromId FROM Followers WHERE FromName=$1;", name)
	err = row.Scan(&id)
	if err != nil {
		url := "https://api.twitch.tv/helix/users?login=" + name
		req, _ := http.NewRequest("GET", url, nil)
		req.Header.Set("Authorization", "Bearer "+strings.Split(os.Getenv("TWITCH_OAUTH_ENV"), ":")[1])
		res, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return
		}
		decoder := json.NewDecoder(res.Body)
		var iddata IdData
		err = decoder.Decode(&iddata)
		if err != nil {
			fmt.Println(err)
			return
		}
		id = iddata.Data[0].ID
	}
	url := "https://api.twitch.tv/helix/users/follows?first=100&from_id=" + id
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))

	res, err := client.Do(req)

	if err != nil {
		fmt.Println(err)
		return
	}
	decoder := json.NewDecoder(res.Body)
	var followers Followers
	err = decoder.Decode(&followers)
	if err != nil {
		fmt.Println(err)
		return
	}

	cursor := followers.Pagination.Cursor
	for i := 0; i < followers.Total/100; i++ {
		url = "https://api.twitch.tv/helix/users/follows?first=100&from_id=" + id + "&after=" + cursor
		req, _ = http.NewRequest("GET", url, nil)
		req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))

		res, err = client.Do(req)

		if err != nil {
			fmt.Println(err)
			return
		}
		decoder = json.NewDecoder(res.Body)
		var temp Followers
		err = decoder.Decode(&temp)
		if err != nil {
			fmt.Println(err)
			return
		}
		followers.Data = append(followers.Data, temp.Data...)
		cursor = temp.Pagination.Cursor
	}

	_, err = tx.Exec("CREATE TEMPORARY TABLE Follow(Id INTEGER PRIMARY KEY, FromId TEXT, FromName TEXT, ToId TEXT, ToName TEXT, FollowedAt TEXT);")

	if err != nil {
		fmt.Println(err)
	}

	for _, d := range followers.Data {
		_, err := tx.Exec("INSERT INTO temp.Follow(FromId, FromName, ToId, ToName, FollowedAt) values($1, $2, $3, $4, $5);", d.FromID, d.FromName, d.ToID, d.ToName, d.FollowedAt)
		if err != nil {
			fmt.Println("temp table")
			fmt.Println(err)
		}
	}

	rows, err := tx.Query("SELECT * FROM temp.Follow tm WHERE NOT EXISTS (SELECT * FROM Followers t WHERE tm.FromId = t.FromId AND tm.ToId = t.Toid);")

	defer rows.Close()

	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("New channels: ")
	for rows.Next() {
		var idx int
		var fromId string
		var fromName string
		var toId string
		var toName string
		var followersAt string
		err = rows.Scan(&idx, &fromId, &fromName, &toId, &toName, &followersAt)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Print(toName + " ")
		_, err = tx.Exec("INSERT INTO Followers(FromId, FromName, ToId, ToName, FollowedAt) values($1, $2, $3, $4, $5);", fromId, fromName, toId, toName, followersAt)
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("")

	rows, err = tx.Query("SELECT t.Id, t.ToName FROM Followers t WHERE t.FromId=$1 AND NOT EXISTS (SELECT * FROM temp.Follow tm WHERE tm.ToId=t.ToId);", id)

	defer rows.Close()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Unfollowed channels: ")
	for rows.Next() {
		var idx int
		var toName string
		err := rows.Scan(&idx, &toName)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Print(toName + " ")
		_, err = tx.Exec("DELETE FROM Followers WHERE id=$1", strconv.Itoa(idx))
		if err != nil {
			fmt.Println(err)
		}
	}
	fmt.Println("")

	rows, err = tx.Query("SELECT ToName FROM Followers WHERE FromId=$1;", id)

	defer rows.Close()
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Currently watching: ")
	for rows.Next() {
		var toName string
		rows.Scan(&toName)
		url = "https://tmi.twitch.tv/group/user/" + strings.ToLower(toName) + "/chatters"

		req, _ := http.NewRequest("GET", url, nil)
		res, err := client.Do(req)

		if err != nil {
			fmt.Println(err)
			continue
		}

		decoder := json.NewDecoder(res.Body)

		var chatData ChatData
		err = decoder.Decode(&chatData)

		if err != nil {
			//fmt.Println(toName)
			//fmt.Println(err)
			continue
		}

		for _, cname := range chatData.Chatters.Vips {
			if cname == name {
				fmt.Print(toName + " ")
			}
		}

		for _, cname := range chatData.Chatters.Moderators {
			if cname == name {
				fmt.Print(toName + " ")
			}
		}

		for _, cname := range chatData.Chatters.Viewers {
			if cname == name {
				fmt.Print(toName + " ")
			}
		}
	}
	fmt.Println("")

	tx.Commit()
	fmt.Println("-----------------------")
}

func parseCommand(str string) {
	args := strings.Split(str, " ")
	if len(args) < 2 {
		fmt.Println("Not enough args")
	}

	command := args[0]
	args = args[1:]

	switch command {
	case "connect":
		if len(args) != 1 {
			fmt.Println("Provide channel name")
			break
		}
		go StartBot(args[0])
	case "find":
		if len(args) != 1 {
			fmt.Println("Provide channel name")
			break
		}

		findPerson(args[0])
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for {
		scanner.Scan()
		args := scanner.Text()
		parseCommand(args)
	}
}
