package logsparser

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

const Layout = "2006-01-02 15:04:05 -0700 MST"

func ParseTime(start, end string) (time.Time, time.Time, error) {
	timeStart, err := time.Parse(Layout, strings.TrimSpace(start)+":00 +0300 MSK")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	timeEnd, err := time.Parse(Layout, strings.TrimSpace(end)+":00 +0300 MSK")
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	return timeStart, timeEnd, nil
}

func Parse(str, msg, username string, timeStart, timeEnd time.Time) ([]string, error) {
	re := regexp.MustCompile(`\[(.*?)\] (.*?): (.*)`)
	match := re.FindStringSubmatch(str)
	timeq, err := time.Parse(Layout, match[1])
	if err != nil {
		return nil, err
	}
	if timeq.Before(timeEnd) && timeq.After(timeStart) {
		if msg != "" {
			if strings.Contains(match[3], msg) {
				return match, nil
			}
		} else if username == match[2] || username == "all" {
			return match, nil
		}
	}
	return nil, errors.New("Nothing was found")
}
