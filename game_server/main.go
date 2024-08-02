package main

import (
	"log"
	"log/slog"
	"net"
	"os"

	"github.com/panshiqu/golang/logger"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/game_server/frame"
	"github.com/panshiqu/server/pb"
	"google.golang.org/grpc"
)

func main() {
	l, err := net.Listen("tcp", ":60001")
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}

	logger.Init()

	s := grpc.NewServer()

	pb.RegisterNetworkServer(s, &frame.NetworkServer{})

	go func() {
		utils.WaitSignal(os.Interrupt)

		frame.Stop()

		s.GracefulStop()
	}()

	slog.Info("serve", slog.Any("err", s.Serve(l)))
}
