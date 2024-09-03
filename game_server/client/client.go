// Can be compared with gate_server/client
package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// 自动
var Auto bool

// 用户编号
var UserID int64

var stream pb.Network_ConnectClient

func Send(cmd pb.Cmd, m proto.Message) {
	if cmd != pb.Cmd_Print {
		log.Println("Send", cmd, m)
	}

	data, err := proto.Marshal(m)
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}

	if err := stream.Send(pb.NewMsg(cmd, data)); err != nil {
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
	var ip = flag.String("ip", "127.0.0.1", "ip")

	flag.BoolVar(&Auto, "auto", false, "automatic")

	flag.Parse()

	log.Println("uid:", *uid)
	log.Println("rid:", *rid)
	log.Println("seat:", *seat)
	log.Println("name:", *name)
	log.Println("auto:", Auto)
	log.Println("print:", *print)
	log.Println("ip:", *ip)

	conn, err := grpc.NewClient(fmt.Sprintf("%s:60001", *ip), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}
	defer conn.Close()

	client := pb.NewNetworkClient(conn)

	md := metadata.Pairs("user_id", *uid, "room_id", *rid, "seat", *seat, "game_name", *name)
	if *print {
		md.Set("print", "true")
	}
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	stream, err = client.Connect(ctx)
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}

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
			} else if s != "" {
				onInput(s)
			}
		}
	}()

	go func() {
		utils.WaitSignal(os.Interrupt)

		log.Println("close send", stream.CloseSend())
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(utils.Wrap(err))
		}

		switch in.Cmd {
		case pb.Cmd_Error:
			Recv(in.Cmd, in.Data, &pb.ErrorResponse{})

		case pb.Cmd_Disconnect:
			log.Println("close send", stream.CloseSend())

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
