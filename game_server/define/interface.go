package define

import (
	"log/slog"
	"time"

	"github.com/panshiqu/golang/pb"
	"github.com/panshiqu/golang/timer"
	"google.golang.org/protobuf/proto"
)

type IRoom interface {
	Logger() *slog.Logger

	IsNobody() bool

	// 游戏遍历用户
	LenUsers() int
	GetUser(int) IUser

	Disband()

	Send(*pb.Msg)
	SendPb(pb.Cmd, proto.Message) error

	Add(time.Duration, func(...any) error, ...any) *timer.Timer
	AddRepeat(time.Duration, func(...any) error, ...any) *timer.Timer
}

type IUser interface {
	ID() int64
	Seat() int
	IsOnline() bool
	Logger() *slog.Logger

	IsWatcher() bool

	StandUp(int)

	Disband()

	Disconnect(int)

	Send(*pb.Msg) error
	SendPb(pb.Cmd, proto.Message) error
}

type IGame interface {
	Init(IRoom) error

	Reconnect(IUser)

	SitDown(IUser)

	StandUp(IUser, int) bool

	OnDisband(int64)

	OnMessage(IUser, *pb.Msg) error
}
