package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"sync"
	"syscall"

	"github.com/coder/websocket"
	"github.com/golang-jwt/jwt/v5"
	"github.com/panshiqu/golang/logger"
	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/config"
	"github.com/panshiqu/server/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
)

var wg sync.WaitGroup

func serve(w http.ResponseWriter, r *http.Request) {
	wg.Add(1)
	defer wg.Done()

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		slog.Error("accept", slog.Any("err", err))
		return
	}
	defer c.CloseNow()

	id, err := jwtParse(r.FormValue("token"))
	if err != nil {
		slog.Error("jwt", slog.Any("err", err))
		return
	}

	ses := &Session{
		id:     id,
		logger: slog.With("id", id),
		conn:   c,
	}

	// 连接即订阅
	addSession(ses)
	defer delSession(ses)

	ses.logger.Debug("enter serve")
	defer ses.logger.Debug("exit serve")

	for {
		typ, data, err := c.Read(r.Context())
		if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
			return
		}
		if err != nil {
			ses.logger.Error("read", slog.Any("err", err))
			return
		}

		if typ != websocket.MessageBinary || len(data) < 4 {
			ses.logger.Error("read", slog.Any("typ", typ), slog.Int("len", len(data)))
			return
		}

		cmd := pb.Cmd(binary.BigEndian.Uint32(data))

		ses.logger.Debug("read", slog.Any("cmd", cmd))

		switch cmd {
		case pb.Cmd_SitDown:
			// 情况不对直接重连
			if ses.stream != nil {
				ses.logger.Error("stream not nil")
				return
			}

			go connect(r.Context(), ses, data[4:])

		case pb.Cmd_StandUp:
			if stream := ses.stream; stream != nil {
				ses.logger.Debug("close stream", slog.Any("err", stream.CloseSend()))
			}

		default:
			if err := ses.Send(cmd, data[4:]); err != nil {
				ses.logger.Error("send", slog.Any("err", err))
				return
			}
		}
	}
}

func connect(ctx context.Context, ses *Session, data []byte) (err error) {
	wg.Add(1)
	defer wg.Done()

	ses.logger.Debug("enter connect")
	defer ses.logger.Debug("exit connect")

	// TODO recover

	defer func() {
		if err != nil {
			ses.logger.Error("connect", slog.Any("err", err))

			ses.Error(utils.Wrap(ses.WritePb(ctx, pb.Cmd_Error, pb.E2er(err, config.IsDev()))), "error response")
		}
	}()

	req := &pb.SitDownRequest{}
	if err := proto.Unmarshal(data, req); err != nil {
		return utils.Wrap(err)
	}

	address := fmt.Sprintf("%s:60001", req.Metadata["game_name"])

	// TODO load online from redis

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return utils.Wrap(err)
	}
	defer conn.Close()

	md := metadata.New(req.Metadata)
	md.Set("user_id", fmt.Sprint(ses.id))
	oc := metadata.NewOutgoingContext(ctx, md)
	stream, err := pb.NewNetworkClient(conn).Connect(oc)
	if err != nil {
		return utils.Wrap(err)
	}

	ses.stream = stream
	defer func() {
		ses.stream = nil
	}()

	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return utils.Wrap(err)
		}

		if in.Cmd == pb.Cmd_Disconnect {
			ses.logger.Debug("passive close stream", slog.Any("err", stream.CloseSend()))
		}

		if err := ses.Write(ctx, in); err != nil {
			return utils.Wrap(err)
		}
	}
}

// -ldflags "-X main.version=v1.1"
var version string

func main() {
	l, err := net.Listen("tcp", ":60006")
	if err != nil {
		log.Fatal(utils.Wrap(err))
	}

	logger.Init()

	config.Init(version)

	s := &http.Server{}

	http.HandleFunc("/", serve)

	go func() {
		utils.WaitSignal(os.Interrupt, syscall.SIGTERM)

		slog.Info("shutdown", slog.Any("err", s.Shutdown(context.Background())))

		Stop()
	}()

	slog.Info("serve", slog.Any("err", s.Serve(l)))

	wg.Wait()
}

// -= Session =-

var rwmutex sync.RWMutex

// 用户-会话，支持对已连接即订阅的用户推送消息譬如充值
// 用户多个设备连接保留所有会话，遍历关闭从而优雅停服
var sessions map[int64][]*Session

func init() {
	sessions = make(map[int64][]*Session)
}

func addSession(ses *Session) {
	rwmutex.Lock()
	sessions[ses.id] = append(sessions[ses.id], ses)
	rwmutex.Unlock()
}

func delSession(ses *Session) {
	rwmutex.Lock()
	defer rwmutex.Unlock()
	sessions[ses.id] = slices.DeleteFunc(sessions[ses.id], func(s *Session) bool {
		return s == ses
	})
}

func Stop() {
	rwmutex.RLock()
	defer rwmutex.RUnlock()
	for _, s := range sessions {
		for _, v := range s {
			v.Error(utils.Wrap(v.conn.CloseNow()), "close")
		}
	}
}

// 每个会话最多有两个协程
// C: 阻塞读客户端消息
// S: 连接游戏服后阻塞读消息
type Session struct {
	// 全局只读仅创建时赋值
	id     int64
	logger *slog.Logger
	conn   *websocket.Conn

	// 协程C只读，使用前总是赋给临时变量后再判空
	stream pb.Network_ConnectClient
}

func (s *Session) Error(err error, msg string, args ...any) {
	logger.Error(err, s.logger, msg, args...)
}

func (s *Session) Send(cmd pb.Cmd, data []byte) error {
	stream := s.stream
	if stream == nil {
		return errors.New("stream is nil")
	}
	return utils.Wrap(stream.Send(pb.NewMsg(cmd, data)))
}

func (s *Session) Write(ctx context.Context, m *pb.Msg) error {
	w, err := s.conn.Writer(ctx, websocket.MessageBinary)
	if err != nil {
		return utils.Wrap(err)
	}
	defer w.Close()

	if err := binary.Write(w, binary.BigEndian, m.Cmd); err != nil {
		return utils.Wrap(err)
	}
	if _, err := w.Write(m.Data); err != nil {
		return utils.Wrap(err)
	}

	return nil
}

func (s *Session) WritePb(ctx context.Context, cmd pb.Cmd, m proto.Message) error {
	data, err := proto.Marshal(m)
	if err != nil {
		return utils.Wrap(err)
	}
	return utils.Wrap(s.Write(ctx, pb.NewMsg(cmd, data)))
}

// -= Function =-

func jwtParse(s string) (int64, error) {
	token, err := jwt.Parse(s, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return base64.StdEncoding.DecodeString(os.Getenv("JWT_KEY"))
	})
	if err != nil {
		return 0, utils.Wrap(err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("claims assert")
	}
	id, ok := claims["id"].(float64)
	if !ok {
		return 0, errors.New("id assert")
	}
	return int64(id), nil
}
