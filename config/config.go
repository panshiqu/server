package config

import (
	"cmp"
	"log/slog"
	"time"
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

func Init(version string) {
	args := make([]any, 0, 4)
	if version != "" {
		args = append(args, slog.String("version", version))
	}

	args = append(args, slog.String("env", cmp.Or(base.Env, "dev")))

	args = append(args, slog.Int("year", time.Now().Year()))

	slog.Info("init", args...)
}
