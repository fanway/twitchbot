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

func SetTerm() {
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
		*buffer = PersonsList(prefix)
		if len(*buffer) == 0 {
			*buffer = append(*buffer, args[0])
		}
		*state += (*buffer)[*tabCount]
	}
}

func createPrefixBuffer(state string, console *Console) []string {
	var prefixBuffer []string
	prefixMap := make(map[string]bool)
	for _, s := range console.commandsBuffer {
		if state == "" || strings.HasPrefix(s, state) && !prefixMap[s] {
			prefixBuffer = append(prefixBuffer, s)
			prefixMap[s] = true
		}
	}
	prefixBuffer = append(prefixBuffer, state)
	return prefixBuffer
}

func (console *Console) processConsole() (string, int) {
	var state string
	var tabBuffer []string
	var tabCount int
	var prefixBuffer []string
	var arrowPointer int
	var arrowState string
	for {
		// \033[H
		fmt.Print("\033[2K\r" + "[" + console.currentChannel + "]> " + state + arrowState)
		ch := getChar(os.Stdin)
		switch ch {
		case BACKSPACE:
			if len(state) > 0 {
				n := len(state) - arrowPointer - 1
				if n >= 0 {
					state = state[:n] + state[n+1:]
					arrowState = ""
					// TODO: rethink this part
					for i := 0; i < arrowPointer; i++ {
						arrowState += "\033[D"
					}
				}
			}
			tabBuffer = tabBuffer[:0]
			prefixBuffer = prefixBuffer[:0]
			tabCount = 0
		case ENTER:
			if len(console.commandsBuffer) == 0 || state != console.commandsBuffer[len(console.commandsBuffer)-1] {
				console.commandsBuffer = append(console.commandsBuffer, state)
			}
			console.commandsBufferCounter = len(console.commandsBuffer)
			fmt.Println("")
			return state, ENTER
		case TAB:
			processTab(&state, &tabBuffer, &tabCount)
		case ESC:
			tempFirst := getChar(os.Stdin)
			if tempFirst == '[' {
				tempSecond := getChar(os.Stdin)
				switch tempSecond {
				case 'A':
					if len(console.commandsBuffer) > 0 {
						if len(prefixBuffer) == 0 {
							prefixBuffer = createPrefixBuffer(state, console)
							console.commandsBufferCounter = len(prefixBuffer) - 1
						}
						//up
						if console.commandsBufferCounter != 0 {
							console.commandsBufferCounter--
							state = prefixBuffer[console.commandsBufferCounter]
						}
					}
				case 'B':
					if len(console.commandsBuffer) > 0 {
						if len(prefixBuffer) == 0 {
							prefixBuffer = createPrefixBuffer(state, console)
							console.commandsBufferCounter = len(prefixBuffer) - 1
						}
						//down
						if console.commandsBufferCounter >= len(prefixBuffer)-1 {
							state = prefixBuffer[len(prefixBuffer)-1]
						} else {
							console.commandsBufferCounter++
							state = prefixBuffer[console.commandsBufferCounter]
						}
					}
				case 'D':
					if arrowPointer < len(state) {
						//left
						arrowPointer++
						arrowState += "\033[D"
					}
				case 'C':
					if arrowPointer > 0 {
						//right
						arrowPointer--
						arrowState += "\033[C"
					}
				}
			} else {
				state += string(tempFirst)
			}
		default:
			n := len(state) - arrowPointer - 1
			state = state[:n+1] + string(ch) + state[n+1:]
			prefixBuffer = prefixBuffer[:0]
		}
	}
}
