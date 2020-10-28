package request

import (
	"encoding/json"
	"errors"
	"image"
	"log"
	"net/http"
	"strconv"
	"time"
	"twitchStats/asciify"
)

func JSON(req *http.Request, timeout int, obj interface{}) error {
	client := &http.Client{Timeout: time.Second * time.Duration(timeout)}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode < http.StatusOK || res.StatusCode > http.StatusIMUsed {
		return errors.New("HTTP status:" + strconv.Itoa(res.StatusCode))
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(res.Body)
	decoder.UseNumber()
	err = decoder.Decode(&obj)
	if err != nil {
		return err
	}
	return nil
}

func Asciify(url string, width int, reverse bool, thMult float32) (string, error) {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", url, nil)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	img, _, err := image.Decode(res.Body)
	if err != nil {
		log.Println(err)
	}
	return asciify.Braille(img, width, reverse, thMult), nil
}
