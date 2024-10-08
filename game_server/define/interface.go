package define

import (
	"bytes"
	"log/slog"
	"time"

	"github.com/panshiqu/golang/timer"
	"github.com/panshiqu/server/pb"
	"google.golang.org/protobuf/proto"
)

type IRoom interface {
	Logger() *slog.Logger

	IsNobody() bool

	// 游戏遍历用户
	LenUsers() int
	GetUser(int) IUser

	Disband()

	Send(*pb.Msg, ...int64)
	SendPb(pb.Cmd, proto.Message, ...int64) error

	Add(time.Duration, func(...any) error, ...any) *timer.Timer
	AddRepeat(time.Duration, func(...any) error, ...any) *timer.Timer
}

type IUser interface {
	ID() int64
	Seat() int
	Data() any
	IsOnline() bool
	Logger() *slog.Logger

	IsWatcher() bool

	SetData(any)

	Error(error, string, ...any)

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

	Print(*bytes.Buffer)
}
