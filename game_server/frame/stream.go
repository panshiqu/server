package frame

import (
	"io"

	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/pb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type NetworkServer struct {
	pb.UnimplementedNetworkServer
}

func (s *NetworkServer) Connect(stream pb.Network_ConnectServer) error {
	md, ok := metadata.FromIncomingContext(stream.Context())
	if !ok {
		return status.Errorf(codes.DataLoss, "failed to get metadata")
	}

	if len(md.Get("print")) > 0 {
		// 更详细的信息请进入房间后无锁打印
		// 仅打印有哪些房间，哪些用户正在哪个房间
		return status.Error(codes.Aborted, Print())
	}

	uid, err := pb.MetadataInt[int64](md, "user_id")
	if err != nil {
		return utils.Wrap(err)
	}

	rid, err := pb.MetadataInt[int64](md, "room_id")
	if err != nil {
		return utils.Wrap(err)
	}

	seat, err := pb.MetadataInt[int](md, "seat")
	if err != nil {
		return utils.Wrap(err)
	}

	name, err := pb.MetadataString(md, "game_name")
	if err != nil {
		return utils.Wrap(err)
	}

	room, err := NewRoom(rid, name)
	if err != nil {
		return utils.Wrap(err)
	}

	user := NewUser(uid)

	if user.room.CompareAndSwap(nil, room) {
		user.SetLogger(room.logger)
	} else {
		room = user.Room()
	}

	room.chSitDown <- &SitDown{
		user:   user,
		stream: stream,
		seat:   seat,
	}

	defer func() {
		room.chStandUp <- &StandUp{
			user:   user,
			stream: stream,
		}
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return utils.Wrap(err)
		}

		room.chUserMsg <- &UserMsg{
			user: user,
			msg:  in,
		}
	}
}
