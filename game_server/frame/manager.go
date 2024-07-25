package frame

import (
	"sync"

	"github.com/panshiqu/golang/utils"
	"github.com/panshiqu/server/game_server/define"
)

// 房间
var rMtx sync.Mutex
var rooms map[int64]*Room

// 停服后不创建房间
var stopped bool

// 等待所有房间协程
var wgRoom sync.WaitGroup

// 用户
var uMtx sync.Mutex
var users map[int64]*User

func init() {
	rooms = make(map[int64]*Room)
	users = make(map[int64]*User)
}

func NewRoom(id int64, name string) (*Room, error) {
	rMtx.Lock()
	defer rMtx.Unlock()

	r, ok := rooms[id]
	if ok {
		return r, nil
	}

	if stopped {
		return nil, define.ErrServerStopped
	}

	r = &Room{
		id: id,
	}
	rooms[id] = r

	if err := r.Init(name); err != nil {
		return nil, utils.Wrap(err)
	}

	go r.routine()

	return r, nil
}

func DelRoom(id int64) {
	rMtx.Lock()
	delete(rooms, id)
	rMtx.Unlock()
}

func Stop() {
	rMtx.Lock()
	stopped = true

	// 解散现有房间
	for _, v := range rooms {
		v.chDisband <- define.DisbandSystem
	}
	rMtx.Unlock()

	wgRoom.Wait()
}

func NewUser(id int64) *User {
	uMtx.Lock()
	defer uMtx.Unlock()

	u, ok := users[id]
	if !ok {
		u = &User{
			id:   id,
			seat: define.InvalidSeat,
		}
		users[id] = u
	}

	return u
}

func DelUser(id int64) {
	uMtx.Lock()
	delete(users, id)
	uMtx.Unlock()
}
