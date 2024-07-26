package game

import (
	"github.com/panshiqu/server/game_server/define"
	"github.com/panshiqu/server/game_server/game/dice"
)

func New(name string) define.IGame {
	switch name {
	case "dice":
		return dice.New()
	}
	return nil
}
