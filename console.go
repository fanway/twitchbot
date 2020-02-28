package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	TAB       = 9
	ENTER     = 10
	BACKSPACE = 127
)

func getChar(f *os.File) byte {
	bs := make([]byte, 1, 1)
	if _, err := f.Read(bs); err != nil {
		return 0
	}
	return bs[0]
}

func setTerm() {
	raw, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TIOCGETA)
	if err != nil {
		//panic(err)
	}
	rawState := *raw
	rawState.Lflag &^= unix.ICANON | unix.ECHO
	err = unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TIOCSETA, &rawState)
	if err != nil {
		//panic(err)
	}
}

func processTab(str *string, buffer *[]string, tabCount *int) {
	args := strings.Split(*str, " ")

	command := args[0]
	args = args[1:]

	*str = command + " "
	if len(*buffer) != 0 {
		*tabCount = (*tabCount + 1) % len(*buffer)
		return
	}
	prefix := "%"
	if len(args) > 0 {
		prefix = args[0] + "%"
	}

	switch command {
	case "find":
		rows := PersonsList(prefix)
		for i := range rows {
			*buffer = append(*buffer, rows[i])
		}
	}
}

func Console() (string, int) {
	var state string
	var buffer []string
	var tabCount int
	setTerm()
	for {
		// \033[H
		fmt.Print("\033[2K\r" + state)
		ch := getChar(os.Stdin)
		switch ch {
		case BACKSPACE:
			if len(state) > 0 {
				state = state[:len(state)-1]
			}
			buffer = buffer[:0]
			tabCount = 0
		case ENTER:
			fmt.Println("")
			return state, ENTER
		case TAB:
			processTab(&state, &buffer, &tabCount)
			state += buffer[tabCount]
		default:
			state += string(ch)
		}
	}
}
