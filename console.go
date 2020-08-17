package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"unicode"

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

func getChar(f *os.File) ([]byte, int) {
	bs := make([]byte, 16, 16)
	n, err := f.Read(bs)
	if err != nil {
		return nil, 0
	}
	return bs, n
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

type Renderer interface {
	render(string, string)
}

type InteractiveRenderer struct {
	comments *[]string
}

func (r InteractiveRenderer) render(state string, arrowState string) {
	fmt.Print("\033[H\033[J")
	for i, _ := range *r.comments {
		if strings.Contains((*r.comments)[i], state) {
			fmt.Print((*r.comments)[i])
		}
	}
	fmt.Print("\033[2K\r" + "> " + state + arrowState)
}

type CoreRenderer struct {
	currentChannel *string
}

func (r CoreRenderer) render(state string, arrowState string) {
	fmt.Print("\033[2K\r" + "[" + *r.currentChannel + "]> " + state + arrowState)
}

func interactiveSort() {
	var comments []string
	console := Console{renderer: InteractiveRenderer{comments: &comments}}
	for {
		state, code := console.processConsole()
		if code == ENTER {
			args := strings.Split(state, " ")
			switch args[0] {
			case "quit":
				return
			case "loadcomments":
				if len(args) <= 1 {
					continue
				}
				var err error
				comments, err = getChatFromVods(args[1])
				if err != nil {
					log.Println(err)
				}
			case "clearcomments":
				if comments == nil {
					log.Println("Load some comments")
					continue
				}
				comments = nil
			}
		}
	}
}

type Console struct {
	commandsBuffer Buffer
	currentChannel string
	comments       []string
	renderer       Renderer
	state          []rune
	arrowState     string
}

func (console *Console) Print(a ...interface{}) {
	fmt.Print("\033[2K\r")
	fmt.Print("\033[u")
	for i, _ := range a {
		if i > 0 {
			fmt.Print(' ')
		}
		fmt.Print(a[i])
	}
	fmt.Print("\033[s")
	fmt.Println()
	fmt.Println()
	console.renderer.render(string(console.state), console.arrowState)
}

func (console *Console) Println(a ...interface{}) {
	fmt.Print("\033[2K\r")
	fmt.Print("\033[u")
	for i, _ := range a {
		if i > 0 {
			fmt.Print(' ')
		}
		fmt.Print(a[i])
	}
	fmt.Println()
	fmt.Print("\033[s")
	fmt.Println()
	console.renderer.render(string(console.state), console.arrowState)
}

func (console *Console) clearState() {
	console.state = []rune{}
	console.arrowState = ""
}

func (console *Console) processConsole() (string, int) {
	var tabBuffer Buffer
	var prefixBuffer Buffer
	var arrowPointer int
	var lenState int
	fmt.Print("\033[s")
	for {
		// \033[H
		lenState = len(console.state)
		strState := string(console.state)
		console.renderer.render(strState, console.arrowState)
		bytes, numOfBytes := getChar(os.Stdin)
		ch := []rune(string(bytes[:numOfBytes]))
		switch ch[0] {
		case BACKSPACE:
			if lenState > 0 {
				n := lenState - arrowPointer - 1
				if n >= 0 {
					console.state = append(console.state[:n], console.state[n+1:]...)
					console.arrowState = ""
					// TODO: rethink this part
					for i := 0; i < arrowPointer; i++ {
						console.arrowState += "\033[D"
					}
				}
			}
			tabBuffer.Clear()
			prefixBuffer.Clear()
		case ENTER:
			if console.commandsBuffer.Empty() || strState != console.commandsBuffer.Back() {
				console.commandsBuffer.Add(strState)
			}
			console.commandsBuffer.index = console.commandsBuffer.Size()
			fmt.Println("")
			console.clearState()
			return strState, ENTER
		case TAB:
			n := lenState - arrowPointer
			left, right := 0, 0
			// find index of the first occurance of '|' character on the left
			for left = n - 1; left > 0; left-- {
				if console.state[left] == '|' {
					// Trim any non letters
					for left < lenState && !unicode.IsLetter(console.state[left]) {
						left++
					}
					break
				}
			}
			// when lenState and arrowPointer equals to 0, for left = n - 1 becomes -1
			// which leads to out of bounds array access
			if left < 0 {
				left = 0
			}
			// find index of the first occurance of '|' character on the right
			for right = n; right < lenState; right++ {
				if console.state[right] == '|' {
					// Trim any non letters
					for !unicode.IsLetter(console.state[right]) {
						right--
					}
					// for convenience to use in the slice ranges
					right++
					break
				}
			}
			console.state = append(console.state[:left], append([]rune(processTab(string(console.state[left:right]), &tabBuffer)), console.state[right:]...)...)
		case ESC:
			if numOfBytes != 3 {
				continue
			}
			if ch[1] == '[' {
				switch ch[2] {
				case ARROW_UP:
					if !console.commandsBuffer.Empty() {
						if prefixBuffer.Empty() {
							prefixBuffer = createPrefixBuffer(strState, &console.commandsBuffer)
						}
						//up
						if prefixBuffer.index != 0 {
							prefixBuffer.index--
							console.state = []rune(prefixBuffer.Get())
						}
					}
				case ARROW_DOWN:
					if !console.commandsBuffer.Empty() {
						if prefixBuffer.Empty() {
							prefixBuffer = createPrefixBuffer(strState, &console.commandsBuffer)
						}
						//down
						if prefixBuffer.index >= prefixBuffer.Size()-1 {
							console.state = []rune(prefixBuffer.Back())
						} else {
							prefixBuffer.index++
							console.state = []rune(prefixBuffer.Get())
						}
					}
				case ARROW_LEFT:
					if arrowPointer < lenState {
						//left
						arrowPointer++
						console.arrowState += "\033[D"
					}
				case ARROW_RIGHT:
					if arrowPointer > 0 {
						//right
						arrowPointer--
						console.arrowState += "\033[C"
					}
				}
			} else {
				console.state = append(console.state, ch[1])
			}
		default:
			n := lenState - arrowPointer - 1
			console.state = append(console.state[:n+1], append(ch, console.state[n+1:]...)...)
			prefixBuffer.Clear()
			tabBuffer.Clear()
		}
	}
}
