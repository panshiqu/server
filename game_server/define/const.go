package define

const (
	InvalidSeat = -1
)

const (
	// 因连接断开而站起
	StandUpDisconnect = iota

	// 结算后站起不在线
	StandUpSettlement
)

const (
	// 被系统解散
	DisbandSystem = iota
)

const (
	// 断开请重试
	DisconnectRetry = iota

	// 解散而断开
	DisconnectDisband

	// 断开老连接
	DisconnectOld
)
