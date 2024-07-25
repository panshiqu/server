package frame

import (
	"log/slog"

	"github.com/panshiqu/golang/pb"
	"github.com/panshiqu/golang/timer"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/game_server/config"
	"github.com/panshiqu/server/game_server/define"
	"github.com/panshiqu/server/game_server/game"
	"google.golang.org/protobuf/proto"
)

type SitDown struct {
	seat   int
	user   *User
	stream pb.Network_ConnectServer
}

type StandUp struct {
	user   *User
	stream pb.Network_ConnectServer
}

type UserMsg struct {
	user *User
	msg  *pb.Msg
}

type Room struct {
	id int64

	users []*User

	chSitDown chan *SitDown
	chStandUp chan *StandUp
	chUserMsg chan *UserMsg
	chDisband chan int64

	game define.IGame

	logger *slog.Logger

	timer.Heap
}

func (r *Room) Init(name string) error {
	n := config.Seat()

	r.users = make([]*User, n, 2*n)

	r.chSitDown = make(chan *SitDown, n)
	r.chStandUp = make(chan *StandUp, n)
	r.chUserMsg = make(chan *UserMsg, 4*n)
	r.chDisband = make(chan int64, n)

	r.logger = slog.With("rid", r.id)

	r.logger.Info("init", slog.String("name", name))

	if r.game = game.New(name); r.game == nil {
		return define.ErrUnsupportGame
	}

	return utils.Wrap(r.game.Init(r))
}

func (r *Room) Logger() *slog.Logger { return r.logger }

func (r *Room) IsNobody() bool {
	for _, v := range r.users {
		if v != nil {
			return false
		}
	}
	return true
}

func (r *Room) LenUsers() int { return len(r.users) }
func (r *Room) GetUser(i int) define.IUser {
	if r.users[i] == nil {
		// IUser(nil) == nil
		return nil
	}
	// users[0] = nil -> IUser(users[0]) != nil
	return r.users[i]
}

func (r *Room) Disband() { r.chDisband <- define.DisbandSystem }

func (r *Room) routine() {
	wgRoom.Add(1)
	defer wgRoom.Done()

	r.logger.Info("enter")
	defer r.logger.Info("exit")

	for r.do() {
	}
}

func (r *Room) do() bool {
	// TODO recover

	select {
	case v := <-r.chSitDown:
		v.user.SetStream(v.stream)

		r.SitDown(v.user, v.seat)

	case v := <-r.chStandUp:
		// 新连接已在老连接断开前坐下
		if v.stream != v.user.stream {
			v.user.logger.Info("stand up break")
			break
		}

		r.StandUp(v.user, define.StandUpDisconnect)

		v.user.stream = nil

	case id := <-r.chDisband:
		r.OnDisband(id)

		return false

	case v := <-r.chUserMsg:
		v.user.logger.Debug("on message", v.msg.Cmd.Attr())

		if err := r.game.OnMessage(v.user, v.msg); err != nil {
			v.user.logger.Error("on message", v.msg.Cmd.Attr(), slog.Any("err", err))

			if err := v.user.SendPb(pb.Cmd_Error, pb.E2er(err, config.IsDev())); err != nil {
				v.user.logger.Error("error response", slog.Any("err", err))
			}
		}

	case <-r.Check():
		r.logger.Debug("check")
	}

	return true
}

func (r *Room) firstSeat() int {
	for k, v := range r.users {
		if v == nil {
			return k
		}
	}
	r.users = append(r.users, nil)
	return len(r.users) - 1
}

func (r *Room) SitDown(u *User, seat int) {
	u.logger.Info("sit down", slog.Int("seat", seat))

	u.online = true

	// 有座则重连
	if u.seat != define.InvalidSeat {
		r.game.Reconnect(u)
		return
	}

	// 没有指定或指定座位有人则找座
	if seat == define.InvalidSeat || r.users[seat] != nil {
		seat = r.firstSeat()
	}

	r.users[seat] = u
	u.seat = seat

	r.game.SitDown(u)
}

func (r *Room) StandUp(u *User, reason int) {
	u.logger.Info("stand up", slog.Int("reason", reason))

	u.online = false

	// 通知游戏站起返回是否可以
	if !r.game.StandUp(u, reason) {
		return
	}

	u.logger.Info("stand up delete")

	r.users[u.seat] = nil
	u.seat = define.InvalidSeat

	DelUser(u.id)
}

func (r *Room) OnDisband(id int64) {
	r.logger.Info("on disband", slog.Int64("id", id))

	r.game.OnDisband(id)

	for _, v := range r.users {
		if v != nil {
			v.Disconnect(define.DisconnectDisband)

			DelUser(v.id)
		}
	}

	DelRoom(r.id)
}

func (r *Room) Send(msg *pb.Msg) {
	for _, v := range r.users {
		if v == nil {
			continue
		}

		if err := v.Send(msg); err != nil {
			v.logger.Error("send", msg.Cmd.Attr(), slog.Any("err", err))
		}
	}
}

func (r *Room) SendPb(cmd pb.Cmd, m proto.Message) error {
	r.logger.Debug("send pb", cmd.Attr(), slog.Any("m", m))

	data, err := proto.Marshal(m)
	if err != nil {
		return utils.Wrap(err)
	}

	r.Send(pb.NewMsg(cmd, data))

	return nil
}
