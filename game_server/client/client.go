package client

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/panshiqu/golang/pb"
	"github.com/panshiqu/golang/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

// 用户编号
var UserID int64

func Send(stream pb.Network_ConnectClient, cmd pb.Cmd, m proto.Message) {
	log.Println("Send", cmd, m)

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

	log.Println("Recv", cmd, m)
}

func Start(onInput func(pb.Network_ConnectClient, string), onMessage func(pb.Network_ConnectClient, *pb.Msg)) {
	var uid = flag.String("u", "1", "user id")
	var rid = flag.String("r", "1", "room id")
	var seat = flag.String("seat", "-1", "seat")
	var name = flag.String("name", "dice", "game name")

	flag.Parse()

	conn, err := grpc.NewClient(":60001", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}
	defer conn.Close()

	client := pb.NewNetworkClient(conn)

	md := metadata.Pairs("user_id", *uid, "room_id", *rid, "seat", *seat, "game_name", *name)
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	stream, err := client.Connect(ctx)
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
				if err.Error() != "unexpected newline" {
					log.Fatal(utils.Wrap(err))
				}
			}
			if s != "" {
				onInput(stream, s)
			}
		}
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Fatal(utils.Wrap(err))
		}

		switch in.Cmd {
		case pb.Cmd_Error:
			Recv(in.Cmd, in.Data, &pb.ErrorResponse{})

		case pb.Cmd_Disconnect:
			log.Println("close send", stream.CloseSend())

		default:
			onMessage(stream, in)
		}
	}
}
