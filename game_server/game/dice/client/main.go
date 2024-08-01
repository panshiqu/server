package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/panshiqu/golang/pb"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/game_server/client"
	"github.com/panshiqu/server/game_server/game/dice"
)

func onInput(stream pb.Network_ConnectClient, s string) {
	switch {
	case strings.HasPrefix(s, "shake"):
		var n int32
		if _, err := fmt.Sscanf(s, "shake,%d", &n); err != nil {
			log.Println(utils.Wrap(err))
		}
		client.Send(stream, pb.Cmd_DiceShake, pb.NewInt32(n))

	default:
		log.Println("unknown input:", s)
	}
}

func onMessage(stream pb.Network_ConnectClient, m *pb.Msg) {
	switch m.Cmd {
	case pb.Cmd_DiceScene:
		notice := &dice.SceneNotice{}
		client.Recv(m.Cmd, m.Data, notice)

		// 已摇过、非自动、非游戏中、时间不够
		if _, ok := notice.Dice[client.UserID]; ok || !client.Auto || !notice.Status || notice.LeftTime < 100 {
			return
		}

		time.AfterFunc(time.Duration(rand.Int63n(notice.LeftTime/2))*time.Millisecond, func() {
			client.Send(stream, pb.Cmd_DiceShake, nil)
		})

	case pb.Cmd_DiceStart:
		n := &pb.Int64{}
		client.Recv(m.Cmd, m.Data, n)

		if !client.Auto {
			return
		}

		time.AfterFunc(time.Duration(rand.Int63n(n.V/2))*time.Millisecond, func() {
			client.Send(stream, pb.Cmd_DiceShake, nil)
		})

	case pb.Cmd_DiceShake:
		client.Recv(m.Cmd, m.Data, &dice.ShakeResponse{})

	case pb.Cmd_DiceSettlement:
		client.Recv(m.Cmd, m.Data, &dice.SettlementNotice{})

	default:
		log.Println("unknown cmd:", m.Cmd)
	}
}

func main() {
	client.Start(onInput, onMessage)
}
