package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"regexp"
	"strconv"
	"time"

	pb "twitchStats/commands/pb"

	"github.com/AllenDang/giu"
	"github.com/AllenDang/giu/imgui"
	"google.golang.org/grpc"
)

var (
	total          int
	layout         giu.Layout
	layoutProgress giu.Layout
	str            []string
	counter        []int
	client         pb.CommandsClient
	quit           chan bool
	status         bool
	channel        string
)

func addInputText() {
	var s string
	str = append(str, s)
	layout = append(layout, giu.InputText(strconv.Itoa(len(str)), 0, &s))
}

func addProgressBar() {
	layoutProgress = layoutProgress[:1]
	for i := range counter {
		fraction := float32(counter[i]) / float32(total)
		layoutProgress = append(layoutProgress, giu.ProgressBar(fraction, -1, 0, fmt.Sprintf("%d (%0.1f%%)", counter[i], fraction*100)))
	}
}

func startVote() {
	status = true
	quit = make(chan bool)
	sendSmartVote()
}

func loop() {
	giu.Window("vote", 10, 30, 400, 200, layout)
	giu.Window("progress", 410, 30, 400, 200, layoutProgress)
}

func sendSmartVote() {
	stream, err := client.ParseAndExec(context.Background(), &pb.Message{
		Channel: channel,
		Text:    "!smartvote 1-" + strconv.Itoa(len(str)),
		Level:   2,
	})
	if err != nil {
		console.Log(err)
		return
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			console.Log(err)
			break
		}
		fmt.Println(in.Text)
		counter = make([]int, len(str))
		go func() {
			ticker := time.NewTicker(time.Millisecond * 100)
			for {
				select {
				case <-quit:
					return
				default:
					sendVoteOptions()
					giu.Update()
					<-ticker.C
					addProgressBar()
				}
			}
		}()
	}
}

func stopVote() {
	if status {
		quit <- true
		status = false
	}
}

func sendVoteOptions() {
	stream, err := client.ParseAndExec(context.Background(), &pb.Message{
		Channel: channel,
		Text:    "!voteoptions",
		Level:   2,
		Status:  "Smartvote",
	})
	if err != nil {
		console.Log(err)
		return
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			console.Log(err)
			break
		}
		re := regexp.MustCompile(`Total votes (\d*)|\((\d*)\)`)
		match := re.FindAllStringSubmatch(in.Text, -1)
		total, err = strconv.Atoi(match[0][1])
		if err != nil {
			break
		}
		for i := range counter {
			counter[i], err = strconv.Atoi(match[i+1][2])
			if err != nil {
				break
			}
		}
	}
}

func main() {
	wnd := giu.NewMasterWindow("Vote", 820, 260, 0, nil)
	layout = append(layout, giu.Line(giu.Button("Add", addInputText), giu.InputText("", 0, &channel)))
	layoutProgress = append(layoutProgress, giu.Line(giu.Button("start vote", startVote), giu.Button("stop vote", stopVote)))
	imgui.StyleColorsDark()
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()}
	grpcConn, err := grpc.Dial("localhost:3434", opts...)
	if err != nil {
		fmt.Println("Unable to connect to grpc")
	}
	client = pb.NewCommandsClient(grpcConn)

	wnd.Main(loop)
}
