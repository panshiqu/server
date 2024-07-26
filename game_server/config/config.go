package config

import (
	"cmp"
)

var base Base

type Base struct {
	Env string

	// 房间可参与游戏的座位数
	Seat int
}

// 开发
func IsDev() bool {
	return base.Env == "" || base.Env == "dev"
}

// 生产
func IsProd() bool {
	return base.Env == "prod"
}

func Seat() int {
	return cmp.Or(base.Seat, 4)
}
