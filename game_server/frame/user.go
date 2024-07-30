package frame

import (
	"log/slog"
	"sync/atomic"

	"github.com/panshiqu/golang/pb"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/game_server/config"
	"github.com/panshiqu/server/game_server/define"
	"google.golang.org/protobuf/proto"
)

type User struct {
	id int64

	seat int

	data any

	online bool

	room atomic.Value

	logger *slog.Logger

	stream pb.Network_ConnectServer
}

// -= Getter =-
func (u *User) ID() int64            { return u.id }
func (u *User) Seat() int            { return u.seat }
func (u *User) Data() any            { return u.data }
func (u *User) IsOnline() bool       { return u.online }
func (u *User) Room() *Room          { return u.room.Load().(*Room) }
func (u *User) Logger() *slog.Logger { return u.logger }

func (u *User) IsWatcher() bool { return u.seat >= config.Seat() }

// -= Setter =-
func (u *User) SetData(d any)            { u.data = d }
func (u *User) SetLogger(l *slog.Logger) { u.logger = l.With("uid", u.id) }

func (u *User) SetStream(s pb.Network_ConnectServer) {
	// 若有断开老连接
	if u.stream != nil {
		u.Disconnect(define.DisconnectOld)
	}

	u.stream = s
}

// -= Function =-
func (u *User) StandUp(reason int) { u.Room().StandUp(u, reason) }
func (u *User) Disband()           { u.Room().chDisband <- u.id }

func (u *User) Disconnect(reason int) {
	u.logger.Info("disconnect", slog.Int("reason", reason))
	if err := u.SendPb(pb.Cmd_Disconnect, pb.NewInt32(reason)); err != nil {
		u.logger.Error("disconnect", slog.Any("err", err))
	}
}

func (u *User) Send(msg *pb.Msg) error {
	if u.stream == nil {
		return define.ErrStreamIsNil
	}

	return utils.Wrap(u.stream.Send(msg))
}

func (u *User) SendPb(cmd pb.Cmd, m proto.Message) error {
	u.logger.Debug("send pb", cmd.Attr(), slog.Any("m", m))

	data, err := proto.Marshal(m)
	if err != nil {
		return utils.Wrap(err)
	}

	return utils.Wrap(u.Send(pb.NewMsg(cmd, data)))
}
