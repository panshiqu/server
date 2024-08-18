// Can be compared with game_server/client
package client

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/coder/websocket"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/pb"
	"google.golang.org/protobuf/proto"
)

// 自动
var Auto bool

// 用户编号
var UserID int64

var conn *websocket.Conn

func Send(cmd pb.Cmd, m proto.Message) {
	if cmd != pb.Cmd_Print {
		log.Println("Send", cmd, m)
	}

	data, err := proto.Marshal(m)
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}

	w, err := conn.Writer(context.Background(), websocket.MessageBinary)
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}
	defer w.Close()

	if err := binary.Write(w, binary.BigEndian, cmd); err != nil {
		log.Fatal(utils.Wrap(err))
	}
	if _, err := w.Write(data); err != nil {
		log.Fatal(utils.Wrap(err))
	}
}

func Recv(cmd pb.Cmd, data []byte, m proto.Message) {
	if err := proto.Unmarshal(data, m); err != nil {
		log.Fatal(utils.Wrap(err))
	}

	if cmd != pb.Cmd_Print {
		log.Println("Recv", cmd, m)
	}
}

func Start(onInput func(string), onMessage func(*pb.Msg)) {
	var uid = flag.String("u", "1", "user id")
	var rid = flag.String("r", "1", "room id")
	var seat = flag.String("seat", "-1", "seat")
	var name = flag.String("name", "dice", "game name")
	var print = flag.Bool("print", false, "print")
	var token = flag.String("token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpZCI6MX0.teQ2o406CHCk91dbp2D3p6ErkfIOELXlyKTkgMiPUT8", "token")

	flag.BoolVar(&Auto, "auto", false, "automatic")

	flag.Parse()

	log.Println("uid:", *uid)
	log.Println("rid:", *rid)
	log.Println("seat:", *seat)
	log.Println("name:", *name)
	log.Println("auto:", Auto)
	log.Println("print:", *print)

	var err error
	conn, _, err = websocket.Dial(context.Background(), fmt.Sprintf("ws://:60006?token=%s", *token), nil)
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}
	defer conn.CloseNow()

	UserID, err = utils.String2Int[int64](*uid)
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}

	go func() {
		for {
			var s string
			if _, err := fmt.Fscanf(os.Stdin, "%s", &s); err != nil {
				log.Println(utils.Wrap(err))
			}

			if s == "print" {
				Send(pb.Cmd_Print, nil)
			} else if s == "sitdown" {
				req := &pb.SitDownRequest{
					Metadata: map[string]string{
						"room_id":   *rid,
						"seat":      *seat,
						"game_name": *name,
					},
				}
				if *print {
					req.Metadata["print"] = "true"
				}
				Send(pb.Cmd_SitDown, req)
			} else if s == "standup" {
				Send(pb.Cmd_StandUp, nil)
			} else if s != "" {
				onInput(s)
			}
		}
	}()

	go func() {
		utils.WaitSignal(os.Interrupt)

		log.Println("close", conn.Close(websocket.StatusNormalClosure, ""))
	}()

	for {
		_, data, err := conn.Read(context.Background())
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			break
		}
		if err != nil {
			log.Fatal(utils.Wrap(err))
		}

		cmd := pb.Cmd(binary.BigEndian.Uint32(data))

		in := pb.NewMsg(cmd, data[4:])

		switch in.Cmd {
		case pb.Cmd_Error:
			Recv(in.Cmd, in.Data, &pb.ErrorResponse{})

		case pb.Cmd_Disconnect:
			Recv(in.Cmd, in.Data, &pb.Int32{})

		case pb.Cmd_SitDown:
			Recv(in.Cmd, in.Data, &pb.User{})

		case pb.Cmd_StandUp:
			Recv(in.Cmd, in.Data, &pb.Int64{})

		case pb.Cmd_Online:
			Recv(in.Cmd, in.Data, &pb.Int64{})

		case pb.Cmd_Print:
			s := &pb.String{}
			Recv(in.Cmd, in.Data, s)
			log.Print(s.V)

		default:
			onMessage(in)
		}
	}
}
