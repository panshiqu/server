
var wg sync.WaitGroup

type server struct {
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

