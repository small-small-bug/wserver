package wserver

import (
	"errors"
	"sync"
	"time"
)

// one command, has several properties
// 1. async or sync
// 2. expire
// 3. need response or not

type CommObject struct {
	id string

	async bool

	start   time.Time
	timeout time.Duration

	request  *CommRequest
	response *CommResponse
	waitCH   chan struct{}

	conn *Conn
}

type CommRequest struct {
	Id  string `json:"id"`
	Msg string `json:"msg"`
}

type CommResponse struct {
	Id  string `json:"id"`
	Msg string `json:"msg"`
}

type CommConn struct {
	conn    *Conn
	commMap map[string]*CommObject
}

type CommManager struct {
	mu              sync.RWMutex
	userConnCommMap map[string]*CommConn
}

func (m *CommManager) Bind(userID string, conn *Conn) error {

	if userID == "" {
		return errors.New("userID can't be empty")
	}

	if conn == nil {
		return errors.New("conn can't be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.userConnCommMap[userID]; ok {
		return errors.New("already registered")
	}
	cc := CommConn{
		conn:    conn,
		commMap: make(map[string]*CommObject),
	}

	var user = userID
	conn.userId = &user
	m.userConnCommMap[userID] = &cc

	return nil
}

// remove all the commands
// leave others to close the connection

// need a way to unbind without user ID
// you cannot get userid in close context

func (m *CommManager) Unbind(conn *Conn) error {

	if conn == nil {
		return errors.New("conn can't be nil")
	}

	var userID *string
	userID = conn.userId

	// the connection is not registered yet.
	if conn.userId == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cc, ok := m.userConnCommMap[*userID]; ok {
		if cc.conn == conn {
			delete(m.userConnCommMap, *userID)
		} else {
			return errors.New("cannot unbind it. it is not yours")
		}
	} else {
		return errors.New("not found")
	}

	return nil
}

func (m *CommManager) hasUser(userId string) (bool, error) {

	if userId == "" {
		return false, errors.New("userID can't be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.userConnCommMap[userId]; ok {
		return true, nil
	}

	return false, nil
}

func (m *CommManager) newCommand(userID, commID string) (*CommObject, error) {

	if userID == "" {
		return nil, errors.New("userID can't be empty")
	}

	if commID == "" {
		return nil, errors.New("userID can't be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cc, ok := m.userConnCommMap[userID]; !ok {
		return nil, errors.New("no such user")
	} else if _, ok := cc.commMap[commID]; ok {
		return nil, errors.New("newCommand: already existed")
	} else {
		comm := CommObject{
			conn:   cc.conn,
			waitCH: make(chan struct{}),
		}

		cc.commMap[commID] = &comm
		return &comm, nil
	}
}

func (m *CommManager) lookupCommand(userID, commID string) (*CommObject, error) {

	if userID == "" {
		return nil, errors.New("userID can't be empty")
	}

	if commID == "" {
		return nil, errors.New("userID can't be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cc, ok := m.userConnCommMap[userID]; !ok {
		return nil, errors.New("no such user")
	} else if c, ok := cc.commMap[commID]; ok {
		return c, nil
	} else {
		return nil, nil
	}
}

func (m *CommManager) removeCommand(userID, commID string) error {

	if userID == "" {
		return errors.New("userID can't be empty")
	}

	if commID == "" {
		return errors.New("userID can't be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// if I cannot find the command, just return OK
	if cc, ok := m.userConnCommMap[userID]; !ok {
		return errors.New("no such user")
	} else if _, ok := cc.commMap[commID]; ok {
		delete(cc.commMap, commID)
		return nil
	} else {
		return errors.New("no such command")
	}
}
