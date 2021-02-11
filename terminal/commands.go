package terminal

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"twitchStats/database"
	"twitchStats/request"
	"twitchStats/statistics"
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

func GetUserId(name string) (string, error) {
	url := "https://api.twitch.tv/helix/users?login=" + name
	req := GetHelixGetRequest(url)
	var iddata IdData
	err := request.JSON(req, 10, &iddata)
	if err != nil {
		Output.Log(err)
		return "", err
	}
	return iddata.Data[0].ID, nil
}

func GetHelixGetRequest(url string) *http.Request {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+strings.Split(os.Getenv("TWITCH_OAUTH_ENV"), ":")[1])
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))
	return req
}

func GetKrakenGetRequest(url string) *http.Request {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Accept", "application/vnd.twitchtv.v5+json")
	req.Header.Set("Authorization", "Bearer "+strings.Split(os.Getenv("TWITCH_OAUTH_ENV"), ":")[1])
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))
	return req
}

func FindPerson(name string) {
	db := database.Connect()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		Output.Log(err)
		return
	}
	defer tx.Rollback()
	var id string
	row := tx.QueryRow("SELECT FromId FROM Followers WHERE FromName=$1;", name)
	err = row.Scan(&id)
	if err != nil {
		var err error
		id, err = GetUserId(name)
		if err != nil {
			Output.Log(err)
			return
		}
	}
	req := GetHelixGetRequest("")
	req.URL, _ = url.Parse("https://api.twitch.tv/helix/users/follows?first=100&from_id=" + id)
	var followers Followers
	err = request.JSON(req, 10, &followers)
	if err != nil {
		Output.Log(err)
		return
	}
	cursor := followers.Pagination.Cursor
	for i := 0; i < followers.Total/100; i++ {
		req.URL, _ = url.Parse("https://api.twitch.tv/helix/users/follows?first=100&from_id=" + id + "&after=" + cursor)
		var temp Followers
		err = request.JSON(req, 10, &temp)
		if err != nil {
			Output.Log(err)
			return
		}
		followers.Data = append(followers.Data, temp.Data...)
		cursor = temp.Pagination.Cursor
	}

	_, err = tx.Exec("CREATE TEMPORARY TABLE Follow(Id INTEGER PRIMARY KEY, FromId TEXT, FromName TEXT, ToId TEXT, ToName TEXT, FollowedAt TEXT);")

	if err != nil {
		Output.Log(err)
	}

	tx.Exec("UPDATE Followers SET FromName=$1 WHERE FromId=$2", followers.Data[0].FromName, id)
	var isLoginName = regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString
	for _, d := range followers.Data {
		if !isLoginName(d.ToName) {
			req.URL, _ = url.Parse("https://api.twitch.tv/helix/users?id=" + d.ToID)
			var m map[string][]map[string]interface{}
			err = request.JSON(req, 10, &m)
			if err != nil {
				Output.Log(err)
				return
			}
			if len(m["data"]) == 0 {
				continue
			}
			d.ToName = m["data"][0]["login"].(string)
		}

		_, err := tx.Exec("INSERT INTO temp.Follow(FromId, FromName, ToId, ToName, FollowedAt) values($1, $2, $3, $4, $5);", d.FromID, d.FromName, d.ToID, d.ToName, d.FollowedAt)
		if err != nil {
			Output.Log(err)
		}
	}

	rows, err := tx.Query("SELECT * FROM temp.Follow tm WHERE NOT EXISTS (SELECT * FROM Followers t WHERE tm.FromId = t.FromId AND tm.ToId = t.Toid);")

	defer rows.Close()

	lowercaseName := strings.ToLower(name)
	m, err := statistics.GetUsers(lowercaseName)
	if err != nil {
		Output.Log(err)
	}
	online := false
	if _, ok := m[lowercaseName]; ok {
		online = true
	}
	if online {
		Output.Println(name + " [ONLINE]" + " [" + time.Now().Format("15:04:05") + "]")
	} else {
		Output.Println(name + " [OFFLINE]" + " [" + time.Now().Format("15:04:05") + "]")
	}
	Output.Println("-----------------------")

	if err != nil {
		Output.Log(err)
	}
	Output.Println("New channels: ")
	for rows.Next() {
		var idx int
		var fromId, fromName, toId, toName, followersAt string
		err = rows.Scan(&idx, &fromId, &fromName, &toId, &toName, &followersAt)
		if err != nil {
			Output.Log(err)
		}
		Output.Print(toName + " ")
		_, err = tx.Exec("INSERT INTO Followers(FromId, FromName, ToId, ToName, FollowedAt) values($1, $2, $3, $4, $5);", fromId, fromName, toId, toName, followersAt)
		if err != nil {
			Output.Log(err)
		}
	}
	Output.Println("")

	rows, err = tx.Query("SELECT t.Id, t.ToName FROM Followers t WHERE t.FromId=$1 AND NOT EXISTS (SELECT * FROM temp.Follow tm WHERE tm.ToId=t.ToId);", id)

	defer rows.Close()
	if err != nil {
		Output.Log(err)
	}

	Output.Println("Unfollowed channels: ")
	for rows.Next() {
		var idx int
		var toName string
		err := rows.Scan(&idx, &toName)
		if err != nil {
			Output.Log(err)
		}
		Output.Print(toName + " ")
		_, err = tx.Exec("DELETE FROM Followers WHERE id=$1", strconv.Itoa(idx))
		if err != nil {
			Output.Log(err)
		}
	}
	Output.Println("")

	rows, err = tx.Query("SELECT ToName FROM Followers WHERE FromId=$1;", id)
	defer rows.Close()
	if err != nil {
		Output.Log(err)
	}
	//if !online {
	//	Output.Println("-----------------------")
	//	tx.Commit()
	//	return
	//}
	Output.Println("Currently watching: ")
	for rows.Next() {
		var toName string
		rows.Scan(&toName)
		m, err := statistics.GetUsers(strings.ToLower(toName))
		if err != nil {
			continue
		}
		if _, ok := m[lowercaseName]; ok {
			Output.Print(toName + " ")
		}
	}
	tx.Commit()
	Output.Println("\n-----------------------")
}

func PersonsList(prefix string) []string {
	db := database.Connect()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		Output.Log(err)
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT DISTINCT FromName FROM Followers WHERE FromName LIKE $1;", prefix)
	defer rows.Close()
	if err != nil {
		Output.Log(err)
	}
	var buffer []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		buffer = append(buffer, s)
	}
	tx.Commit()
	return buffer
}

func CrossFollow(username1, username2 string) []string {
	db := database.Connect()
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		Output.Log(err)
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT ToName FROM Followers WHERE FromName=$1 OR FromName=$2 GROUP BY ToName HAVING COUNT(*) > 1;", username1, username2)
	defer rows.Close()
	if err != nil {
		Output.Log(err)
	}
	var followArr []string
	for rows.Next() {
		var s string
		rows.Scan(&s)
		followArr = append(followArr, s)
	}
	tx.Commit()
	return followArr
}

type VodsChat struct {
	Comments []Comments `json:"comments"`
	Next     string     `json:"_next,omitempty"`
}

func (chat *VodsChat) parse() []string {
	var messages []string
	for _, comment := range chat.Comments {
		username := comment.Commenter.DisplayName
		currentTime := time.Now()
		timeCreated := comment.CreatedAt.In(currentTime.Location()).Format("2006-01-02 15:04:05 -0700 MST")
		timeOffset := time.Date(0, 0, 0, 0, 0, int(comment.ContentOffsetSeconds), 0, time.UTC).Format("15:04:05")
		msg := comment.Message.Body
		messages = append(messages, fmt.Sprintf("[%s] %s: %s [%s]\n", timeCreated, username, msg, timeOffset))
	}
	return messages
}

type Commenter struct {
	DisplayName string      `json:"display_name"`
	ID          string      `json:"_id"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Bio         interface{} `json:"bio"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	Logo        string      `json:"logo"`
}

type UserBadges struct {
	ID      string `json:"_id"`
	Version string `json:"version"`
}

type VodMessage struct {
	Body             string       `json:"body"`
	Emoticons        []Emoticons  `json:"emoticons,omitempty"`
	Fragments        []Fragments  `json:"fragments"`
	IsAction         bool         `json:"is_action"`
	UserBadges       []UserBadges `json:"user_badges,omitempty"`
	UserColor        string       `json:"user_color,omitempty"`
	UserNoticeParams interface{}  `json:"user_notice_params,omitempty"`
}

type Fragments struct {
	Text     string   `json:"text"`
	Emoticon Emoticon `json:"emoticon,omitempty"`
}

type Emoticon struct {
	EmoticonID    string `json:"emoticon_id"`
	EmoticonSetID string `json:"emoticon_set_id"`
}

type Emoticons struct {
	ID    string `json:"_id"`
	Begin int    `json:"begin"`
	End   int    `json:"end"`
}

type Comments struct {
	ID                   string     `json:"_id"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	ChannelID            string     `json:"channel_id"`
	ContentType          string     `json:"content_type"`
	ContentID            string     `json:"content_id"`
	ContentOffsetSeconds float64    `json:"content_offset_seconds"`
	Commenter            Commenter  `json:"commenter"`
	Source               string     `json:"source"`
	State                string     `json:"state"`
	Message              VodMessage `json:"message,omitempty"`
}

func extractId(url string) string {
	re := regexp.MustCompile(`https://www\.twitch\.tv/videos/(.*)`)
	s := re.FindStringSubmatch(url)
	if s != nil {
		return s[1]
	}
	return url
}

func GetChatFromVods(link string) ([]string, error) {
	videoId := extractId(link)
	u := "https://api.twitch.tv/v5/videos/" + videoId + "/comments?cursor="
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Client-ID", os.Getenv("TWITCH_CLIENT_ID"))
	var vodsChat VodsChat
	err := request.JSON(req, 10, &vodsChat)
	if err != nil {
		return nil, err
	}
	messages := vodsChat.parse()
	fmt.Println(messages)
	for vodsChat.Next != "" {
		req.URL, _ = url.Parse(u + vodsChat.Next)
		vodsChat = VodsChat{}
		err := request.JSON(req, 10, &vodsChat)
		if err != nil {
			return nil, err
		}
		newMessages := vodsChat.parse()
		fmt.Println(newMessages)
		messages = append(messages, newMessages...)
	}
	return messages, nil
}
