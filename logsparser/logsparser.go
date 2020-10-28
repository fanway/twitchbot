package logsparser

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

func ParseTime(start, end string) (time.Time, time.Time, error) {
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

func Parse(str, msg, username string, timeStart, timeEnd time.Time) (string, error) {
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
