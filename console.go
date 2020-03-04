package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	TAB        = 9
	ENTER      = 10
	BACKSPACE  = 127
	ESC        = 27
	ARROW_UP   = "\033[A"
	ARROW_DOWN = "\033[B"
)

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

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

func processTab(state *string, buffer *[]string, tabCount *int) {
	args := strings.Split(*state, " ")

	command := args[0]
	args = args[1:]

	*state = command + " "
	if len(*buffer) != 0 {
		*tabCount = (*tabCount + 1) % len(*buffer)
		*state += (*buffer)[*tabCount]
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

		*state += (*buffer)[*tabCount]
	}
}

func Console(arrowBuffer *[]string, arrowCount *int) (string, int) {
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
			*arrowBuffer = append(*arrowBuffer, state)
			*arrowCount = len(*arrowBuffer)
			fmt.Println("")
			return state, ENTER
		case TAB:
			processTab(&state, &buffer, &tabCount)
		case ESC:
			tempFirst := getChar(os.Stdin)
			if tempFirst == '[' {
				if len(*arrowBuffer) == 0 {
					break
				}
				tempSecond := getChar(os.Stdin)
				if tempSecond == 'A' {
					if *arrowCount != 0 {
						*arrowCount--
						state = (*arrowBuffer)[*arrowCount]
					}
				} else if tempSecond == 'B' {
					if *arrowCount == len(*arrowBuffer)-1 {
						state = ""
					} else {
						*arrowCount++
						state = (*arrowBuffer)[*arrowCount]
					}
				}
			} else {
				state += string(tempFirst)
			}
		default:
			state += string(ch)
		}
	}
}
