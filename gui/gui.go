package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"

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
)

func addInputText() {
	var s string
	str = append(str, s)
	layout = append(layout, giu.InputText(strconv.Itoa(len(str)), 0, &s))
}

func startVote() {
	sendSmartVote()
	sendVoteOptions()
}

func loop() {
	/*
		l := giu.Layout{
			giu.Label("Golosovanie"),
			giu.Label(fmt.Sprintf("%d", counter)),
			giu.ProgressBar(float32(counter)/float32(total), -1, 0, strconv.Itoa(counter)),
			giu.ProgressBar(float32(counter2)/float32(total), -1, 0, strconv.Itoa(counter2)),
		}
	*/
	giu.Window("vote", 10, 30, 400, 200, layout)
	giu.Window("progress", 410, 30, 400, 200, layoutProgress)
}

func sendSmartVote() {
	stream, err := client.ParseAndExec(context.Background(), &pb.Message{
		Channel:  "#funwayz",
		Username: "funwayz",
		Text:     "!smartvote 0-" + strconv.Itoa(len(str)),
		Level:    2,
	})
	if err != nil {
		log.Println(err)
		return
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		fmt.Println(in.Text)
	}
}

func sendVoteOptions() {
	stream, err := client.ParseAndExec(context.Background(), &pb.Message{
		Channel:  "#funwayz",
		Username: "funwayz",
		Text:     "!voteoptions",
		Level:    2,
		Status:   "Smartvote",
	})
	if err != nil {
		log.Println(err)
		return
	}
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println(err)
			break
		}
		fmt.Println(in.Text)
	}
}

func main() {
	wnd := giu.NewMasterWindow("Vote", 800, 600, 0, nil)
	layout = append(layout, giu.Button("Add", addInputText))
	layoutProgress = append(layoutProgress, giu.Button("start vote", startVote))
	imgui.StyleColorsDark()
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithBlock()}
	grpcConn, err := grpc.Dial("localhost:3434", opts...)
	if err != nil {
		fmt.Println("Unable to connect to grpc")
	}
	client = pb.NewCommandsClient(grpcConn)

	wnd.Main(loop)
}
