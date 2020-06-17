package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	TAB         = 9
	ENTER       = 10
	BACKSPACE   = 127
	ESC         = 27
	ARROW_UP    = 'A'
	ARROW_DOWN  = 'B'
	ARROW_LEFT  = 'D'
	ARROW_RIGHT = 'C'
)

func abs(x int) int {
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

func processTab(state string, buffer *Buffer) string {
	state = strings.Trim(state, " ")
	args := strings.Split(state, " ")

	command := args[0]
	args = args[1:]

	newState := command + " "
	prefix := "%"
	if len(args) > 0 {
		prefix = args[0] + "%"
	}

	switch command {
	case "find":
		if buffer.Empty() {
			buffer.Append(personsList(prefix))
			if buffer.Empty() {
				buffer.Add(args[0])
			}
		}
		return newState + buffer.Cycle()
	default:
		return newState
	}
}

func createPrefixBuffer(state string, commandsBuffer *Buffer) Buffer {
	var prefixBuffer Buffer
	prefixMap := make(map[string]int)
	for _, s := range commandsBuffer.buffer {
		prefixMap[s]++
	}
	for _, s := range commandsBuffer.buffer {
		prefixMap[s]--
		if state == "" || strings.HasPrefix(s, state) && prefixMap[s] == 0 {
			prefixBuffer.Add(s)
		}
	}
	prefixBuffer.Add(state)
	return prefixBuffer
}

func isLetter(char byte) bool {
	return char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'
}

func (console *Console) processConsole() (string, int) {
	var state string
	var tabBuffer Buffer
	var prefixBuffer Buffer
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
			tabBuffer.Clear()
			prefixBuffer.Clear()
		case ENTER:
			if console.commandsBuffer.Empty() || state != console.commandsBuffer.Back() {
				console.commandsBuffer.Add(state)
			}
			console.commandsBuffer.index = console.commandsBuffer.Size()
			fmt.Println("")
			return state, ENTER
		case TAB:
			n := len(state) - arrowPointer
			left, right := 0, 0
			// find index of the first occurance of '|' character on the left
			for left = n - 1; left > 0; left-- {
				if state[left] == '|' {
					// Trim any non letters
					for left < len(state) && !isLetter(state[left]) {
						left++
					}
					break
				}
			}
			if left == -1 {
				left = 0
			}
			// find index of the first occurance of '|' character on the right
			for right = n; right < len(state); right++ {
				if state[right] == '|' {
					// Trim any non letters
					for !isLetter(state[right]) {
						right--
					}
					// for convenience to use in the slice ranges
					right++
					break
				}
			}
			state = state[:left] + processTab(state[left:right], &tabBuffer) + state[right:]
		case ESC:
			tempFirst := getChar(os.Stdin)
			if tempFirst == '[' {
				tempSecond := getChar(os.Stdin)
				switch tempSecond {
				case ARROW_UP:
					if !console.commandsBuffer.Empty() {
						if prefixBuffer.Empty() {
							prefixBuffer = createPrefixBuffer(state, &console.commandsBuffer)
						}
						//up
						if prefixBuffer.index != 0 {
							prefixBuffer.index--
							state = prefixBuffer.Get()
						}
					}
				case ARROW_DOWN:
					if !console.commandsBuffer.Empty() {
						if prefixBuffer.Empty() {
							prefixBuffer = createPrefixBuffer(state, &console.commandsBuffer)
						}
						//down
						if prefixBuffer.index >= prefixBuffer.Size()-1 {
							state = prefixBuffer.Back()
						} else {
							prefixBuffer.index++
							state = prefixBuffer.Get()
						}
					}
				case ARROW_LEFT:
					if arrowPointer < len(state) {
						//left
						arrowPointer++
						arrowState += "\033[D"
					}
				case ARROW_RIGHT:
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
			prefixBuffer.Clear()
			tabBuffer.Clear()
		}
	}
}
