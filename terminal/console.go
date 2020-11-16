package terminal

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
	"twitchStats/buffer"
	"unicode"

	"golang.org/x/sys/unix"
)

var Output Console

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

func GetChar(f *os.File) ([]byte, int) {
	bs := make([]byte, 16, 16)
	n, err := f.Read(bs)
	if err != nil {
		return nil, 0
	}
	return bs, n
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

func ProcessTab(state string, buffer *buffer.Buffer) string {
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
			buffer.Append(PersonsList(prefix))
			if buffer.Empty() {
				buffer.Add(args[0])
			}
		}
		return newState + buffer.Cycle()
	default:
		return newState
	}
}

func createPrefixBuffer(state string, commandsBuffer *buffer.Buffer) buffer.Buffer {
	var prefixBuffer buffer.Buffer
	prefixMap := make(map[string]int)
	for _, s := range commandsBuffer.Buffer {
		prefixMap[s]++
	}
	for _, s := range commandsBuffer.Buffer {
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
	CurrentChannel *string
}

func (r CoreRenderer) render(state string, arrowState string) {
	fmt.Print("\033[2K\r" + "[" + *r.CurrentChannel + "]> " + state + arrowState)
}

func InteractiveSort() {
	var comments []string
	console := Console{Renderer: InteractiveRenderer{comments: &comments}}
	for {
		state, code := console.ProcessConsole()
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
				comments, err = GetChatFromVods(args[1])
				if err != nil {
					console.Log(err)
				}
			case "clearcomments":
				if comments == nil {
					console.Log("Load some comments")
					continue
				}
				comments = nil
			}
		}
	}
}

type Console struct {
	CommandsBuffer buffer.Buffer
	CurrentChannel string
	Comments       []string
	Renderer       Renderer
	state          []rune
	arrowState     string
	cursorW        int
}

func (console *Console) Print(a ...interface{}) {
	fmt.Print("\033[2K\r")
	if console.cursorW != 0 {
		fmt.Print("\033[2F")
		fmt.Printf("\033[%dC", console.cursorW)
	} else {
		fmt.Print("\033[1F")
	}
	for i, _ := range a {
		if i > 0 {
			fmt.Print(" ")
			console.cursorW += 1
		}
		fmt.Print(a[i])
		console.cursorW += len(a[i].(string))
	}
	fmt.Println()
	fmt.Println()
	console.Renderer.render(string(console.state), console.arrowState)
}

func (console *Console) Println(a ...interface{}) {
	fmt.Print("\033[2K\r")
	if console.cursorW != 0 {
		fmt.Print("\033[2F")
		fmt.Printf("\033[%dC", console.cursorW)
	} else {
		fmt.Print("\033[1F")
	}
	for i, _ := range a {
		if i > 0 {
			fmt.Print(" ")
		}
		fmt.Print(a[i])
	}
	fmt.Println()
	fmt.Println()
	console.cursorW = 0
	console.Renderer.render(string(console.state), console.arrowState)
}

func (console *Console) Log(a ...interface{}) {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	shortFile := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			shortFile = file[i+1:]
			break
		}
	}
	time := time.Now().Format("2006-01-02 15:04:05 -0700 MST")
	var s []interface{}
	s = append(s, fmt.Sprintf("[%s] %s:%d: ", time, shortFile, line))
	s = append(s, a...)
	console.Println(s...)
}

func (console *Console) clearState() {
	console.state = []rune{}
	console.arrowState = ""
	console.cursorW = 0
}

func (console *Console) ProcessConsole() (string, int) {
	var tabBuffer buffer.Buffer
	var prefixBuffer buffer.Buffer
	var arrowPointer int
	var lenState int
	for {
		// \033[H
		lenState = len(console.state)
		strState := string(console.state)
		console.Renderer.render(strState, console.arrowState)
		bytes, numOfBytes := GetChar(os.Stdin)
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
			if console.CommandsBuffer.Empty() || strState != console.CommandsBuffer.Back() {
				console.CommandsBuffer.Add(strState)
			}
			console.CommandsBuffer.Index = console.CommandsBuffer.Size()
			fmt.Println()
			fmt.Println()
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
			console.state = append(console.state[:left], append([]rune(ProcessTab(string(console.state[left:right]), &tabBuffer)), console.state[right:]...)...)
		case ESC:
			if numOfBytes != 3 {
				continue
			}
			if ch[1] == '[' {
				switch ch[2] {
				case ARROW_UP:
					if !console.CommandsBuffer.Empty() {
						if prefixBuffer.Empty() {
							prefixBuffer = createPrefixBuffer(strState, &console.CommandsBuffer)
						}
						//up
						if prefixBuffer.Index != 0 {
							prefixBuffer.Index--
							console.state = []rune(prefixBuffer.Get())
						}
					}
				case ARROW_DOWN:
					if !console.CommandsBuffer.Empty() {
						if prefixBuffer.Empty() {
							prefixBuffer = createPrefixBuffer(strState, &console.CommandsBuffer)
						}
						//down
						if prefixBuffer.Index >= prefixBuffer.Size()-1 {
							console.state = []rune(prefixBuffer.Back())
						} else {
							prefixBuffer.Index++
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
