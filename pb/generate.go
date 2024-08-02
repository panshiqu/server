package pb

import (
	"errors"
	"log/slog"

	"github.com/panshiqu/golang/utils"
)

//go:generate sh -c "protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative *.proto"

func (e *ErrorResponse) Error() string {
	return e.String()
}

func Er(code Err, desc ...string) *ErrorResponse {
	e := &ErrorResponse{
		Code: code,
	}
	if len(desc) > 0 {
		e.Desc = desc[0]
	}
	return e
}

func E2er(err error, dev bool) (er *ErrorResponse) {
	if errors.As(err, &er) {
		if dev {
			er.Detail = utils.FileLine(err)
		}
		return
	}
	er = Er(Err_Fail)
	if dev {
		er.Detail = err.Error()
	}
	return
}

func (c Cmd) Attr() slog.Attr {
	return slog.Any("cmd", c)
}

func NewMsg(cmd Cmd, data []byte) *Msg {
	return &Msg{
		Cmd:  cmd,
		Data: data,
	}
}

func NewInt32[T ~int | ~int32](v T) *Int32 {
	return &Int32{
		V: int32(v),
	}
}

func NewInt64[T ~int | ~int32 | ~int64](v T) *Int64 {
	return &Int64{
		V: int64(v),
	}
}

func NewString(s string) *String {
	return &String{
		V: s,
	}
}
