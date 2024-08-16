package dice

import (
	"bytes"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/panshiqu/golang/logger"
	"github.com/panshiqu/golang/timer"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/config"
	"github.com/panshiqu/server/game_server/define"
	"github.com/panshiqu/server/pb"
	"google.golang.org/protobuf/proto"
)

//go:generate protoc --go_out=. --go_opt=paths=source_relative dice.proto

const (
	// 空闲时长
	IdleDuration = 5 * time.Second

	// 游戏时长
	GameDuration = 30 * time.Second
)

type Data struct {
	// 游戏局数
	count int
}

type Game struct {
	room define.IRoom

	// 状态是否游戏中
	status bool

	// 空闲游戏定时器
	timer *timer.Timer

	// 用户编号骰子点数
	dice map[int64]int32
}

func New() *Game {
	return &Game{}
}

func (g *Game) Init(r define.IRoom) error {
	r.Logger().Info("init")

	g.room = r

	return utils.Wrap(g.start("init"))
}

func (g *Game) sceneNotice(u define.IUser) {
	notice := &SceneNotice{
		Status:   g.status,
		LeftTime: g.timer.Milli(),
		Dice:     g.dice,
	}

	// 游戏中且已有人摇过
	// 屏蔽他人但保留自己点数
	if g.status && len(g.dice) > 0 {
		dice := make(map[int64]int32)
		for k, v := range g.dice {
			if k == u.ID() {
				dice[k] = v
			} else {
				dice[k] = 0
			}
		}
		notice.Dice = dice
	}

	u.Error(utils.Wrap(u.SendPb(pb.Cmd_DiceScene, notice)), "scene notice")
}

func (g *Game) Reconnect(u define.IUser) {
	u.Logger().Info("reconnect")

	g.sceneNotice(u)
}

func (g *Game) SitDown(u define.IUser) {
	u.Logger().Info("sitdown")

	g.sceneNotice(u)

	u.SetData(&Data{})
}

func (g *Game) StandUp(u define.IUser, reason int) bool {
	// 摇过且游戏中不能走
	if _, ok := g.dice[u.ID()]; ok && g.status {
		return false
	}

	u.Logger().Info("standup statistics", slog.Int("count", u.Data().(*Data).count))

	return true
}

func (g *Game) OnDisband(id int64) {
	// 游戏中解散则提前结算
	if g.status {
		logger.Error(utils.Wrap(g.settlement("disband")), g.room.Logger(), "ondisband settlement")
	}
}

func (g *Game) OnMessage(u define.IUser, m *pb.Msg) error {
	switch m.Cmd {
	// 摇骰子
	case pb.Cmd_DiceShake:
		return utils.Wrap(g.shake(u, m.Data))

	// 解散房间
	// u.Disband()

	// 主动断开连接
	// u.Disconnect(define.DisconnectRetry)

	default:
		return utils.Wrap(pb.Er(pb.Err_UnknownCommand, fmt.Sprintf("unknown command: %v", m.Cmd)))
	}
}

func (g *Game) isAllShaken() bool {
	for i := 0; i < config.Seat(); i++ {
		if u := g.room.GetUser(i); u != nil {
			if _, ok := g.dice[u.ID()]; !ok {
				return false
			}
		}
	}
	return true
}

func (g *Game) shake(u define.IUser, data []byte) error {
	if !g.status {
		return utils.Wrap(pb.Er(pb.Err_StatusMismatch, "status mismatch"))
	}

	if u.IsWatcher() {
		return utils.Wrap(pb.Er(pb.Err_WatcherOperate, "watcher can't operate"))
	}

	if _, ok := g.dice[u.ID()]; ok {
		return utils.Wrap(pb.Er(pb.Err_DiceAlreadyShaken, "already shaken the dice"))
	}

	request := &pb.Int32{}
	if err := proto.Unmarshal(data, request); err != nil {
		return utils.Wrap(err)
	}
	u.Logger().Info("shake", slog.Any("m", request))

	n := rand.Int31n(6) + 1

	// 测试环境支持预设骰子点数
	if config.IsDev() && request.V != 0 {
		n = request.V
	}

	// 通知除我之外
	response := &ShakeResponse{
		UserID: u.ID(),
	}
	if err := g.room.SendPb(pb.Cmd_DiceShake, response, u.ID()); err != nil {
		return utils.Wrap(err)
	}

	// 带点数给我回复
	response.DicePoints = n
	if err := u.SendPb(pb.Cmd_DiceShake, response); err != nil {
		return utils.Wrap(err)
	}

	g.dice[u.ID()] = n

	// 多人摇过且非旁观在座的已全摇则提前结算
	if len(g.dice) > 1 && g.isAllShaken() {
		g.timer.Stop()

		u.Error(utils.Wrap(g.settlement("shake")), "shake settlement")
	}

	return nil
}

func (g *Game) start(args ...any) error {
	g.room.Logger().Info("start", slog.Any("from", args))

	g.status = true

	g.dice = make(map[int64]int32)

	g.timer = g.room.Add(GameDuration, g.settlement, "timer")

	return utils.Wrap(g.room.SendPb(pb.Cmd_DiceStart, pb.NewInt64(GameDuration.Milliseconds())))
}

func (g *Game) settlement(args ...any) error {
	g.room.Logger().Info("settlement", slog.Any("from", args))

	g.status = false

	var max int32
	for _, v := range g.dice {
		if v > max {
			max = v
		}
	}

	winner := make([]int64, 0, 2)
	for k, v := range g.dice {
		if v == max {
			winner = append(winner, k)
		}
	}

	if err := g.room.SendPb(pb.Cmd_DiceSettlement, &SettlementNotice{
		IdleDuration: IdleDuration.Milliseconds(),
		Winner:       winner,
		Dice:         g.dice,
	}); err != nil {
		return utils.Wrap(err)
	}

	for i := 0; i < config.Seat(); i++ {
		u := g.room.GetUser(i)
		if u == nil {
			continue
		}

		if _, ok := g.dice[u.ID()]; ok {
			u.Logger().Debug("settlement statistics")

			u.Data().(*Data).count++
		}

		if !u.IsOnline() {
			u.StandUp(define.StandUpSettlement)
		}
	}

	if g.room.IsNobody() {
		g.room.Logger().Info("settlement disband")

		g.room.Disband()
	}

	g.timer = g.room.Add(IdleDuration, g.start, "timer")

	return nil
}

func (g *Game) Print(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "status: %v\ndata:\n", g.status)
	for i := 0; i < g.room.LenUsers(); i++ {
		if u := g.room.GetUser(i); u != nil {
			fmt.Fprintf(buf, " count: %d\n", u.Data().(*Data).count)
		}
	}
	fmt.Fprintln(buf, "dice:")
	for k, v := range g.dice {
		fmt.Fprintf(buf, " %d-%d\n", k, v)
	}
}
