package main

import (
	"bufio"
	"math/rand"
	"os"
	"strings"
	"time"
)

func add(m map[string]map[string]int, first, second string) {
	child, ok := m[first]
	if !ok {
		child = map[string]int{}
		m[first] = child
	}
	child[second]++
}

func Markov(channel string) (string, error) {
	msg := ""
	file, err := os.Open(channel + ".log")
	defer file.Close()
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(file)
	m := make(map[string]map[string]int)
	for scanner.Scan() {
		line := scanner.Text()
		// parse message
		str := strings.Split(strings.Split(line, "]")[1], ": ")[1]
		sp := strings.Split(str, " ")
		if len(sp) < 3 {
			continue
		}
		// add special word
		add(m, "Begin", sp[0])
		add(m, sp[len(sp)-1], "End")
		// add words in message
		for j := 0; j < len(sp)-1; j++ {
			add(m, sp[j], sp[j+1])
		}
	}
	text := []string{"Begin"}
	for {
		if text[len(text)-1] == "End" {
			if len(text) >= 10 {
				break
			}
			text = []string{"Begin"}
		}
		word := text[len(text)-1]
		var randSlice []string
		// take random next word, taking into account the frequency of words
		for k, _ := range m[word] {
			for i := 0; i < m[word][k]; i++ {
				// m[word][k] is the number of times the word "word" comes before the word "k"
				randSlice = append(randSlice, k)
			}
		}
		if len(randSlice) == 0 || len(text) == 100 {
			text = append(text, "End")
			continue
		}
		rand.Seed(time.Now().UnixNano())
		text = append(text, randSlice[rand.Intn(len(randSlice))])
	}
	msg = text[1]
	for i := 2; i < len(text)-1; i++ {
		msg += " " + text[i]
	}
	return msg, nil
}
