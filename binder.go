package wserver

import (
	"errors"
	"fmt"
	"sync"
)

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
		conn: conn,
	}
	m.userConnCommMap[userID] = &cc

	return nil
}

// remove all the commands
// leave others to close the connection
func (m *CommManager) Unbind(userID string, conn *Conn) error {

	if userID == "" {
		return errors.New("userID can't be empty")
	}

	if conn == nil {
		return errors.New("conn can't be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if cc, ok := m.userConnCommMap[userID]; ok {
		if cc.conn == conn {
			delete(m.userConnCommMap, userID)
		} else {
			return errors.New("cannot unbind it. it is not yours")
		}
	} else {
		return errors.New("not found")
	}

	return nil
}

func (m *CommManager) isIn(userId string) (bool, error) {

	if userID == "" {
		return false, errors.New("userID can't be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.userConnCommMap[userId]; ok {
		return true, nil
	}

	return false, nil
}

func (m *CommManager) lookCreate(userID, commID string) (*Comm, error) {
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
		comm := Comm{
			request:  nil,
			response: nil,
			waitCH:   make(chan struct{}),
		}
		cc.commMap[commID] = &comm
		return &comm, nil
	}
}

func (m *CommManager) lookReply(userID, commID string) (*Comm, error) {
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
		comm := Comm{
			request:  nil,
			response: nil,
			waitCH:   make(chan struct{}),
		}
		cc.commMap[commID] = &comm
		return &comm, nil
	}
}

// binder is defined to store the relation of userID and eventConn
type binder struct {
	mu sync.RWMutex

	// map stores key: userID and value of related slice of eventConn
	userID2EventConnMap map[string]*[]eventConn

	// map stores key: connID and value: userID
	connID2UserIDMap map[string]string
}

// Bind binds userID with eConn specified by event. It fails if the
// return error is not nil.
func (b *binder) Bind(userID, event string, conn *Conn) error {
	if userID == "" {
		return errors.New("userID can't be empty")
	}

	if event == "" {
		return errors.New("event can't be empty")
	}

	if conn == nil {
		return errors.New("conn can't be nil")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// map the eConn if it isn't be put.
	if eConns, ok := b.userID2EventConnMap[userID]; ok {
		for i := range *eConns {
			if (*eConns)[i].Conn == conn {
				return nil
			}
		}

		newEConns := append(*eConns, eventConn{event, conn})
		b.userID2EventConnMap[userID] = &newEConns
	} else {
		b.userID2EventConnMap[userID] = &[]eventConn{{event, conn}}
	}
	b.connID2UserIDMap[conn.GetID()] = userID

	return nil
}

// Unbind unbind and removes Conn if it's exist.
func (b *binder) Unbind(conn *Conn) error {
	if conn == nil {
		return errors.New("conn can't be empty")
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// query userID by connID
	userID, ok := b.connID2UserIDMap[conn.GetID()]
	if !ok {
		return fmt.Errorf("can't find userID by connID: %s", conn.GetID())
	}

	if eConns, ok := b.userID2EventConnMap[userID]; ok {
		for i := range *eConns {
			if (*eConns)[i].Conn == conn {
				newEConns := append((*eConns)[:i], (*eConns)[i+1:]...)
				b.userID2EventConnMap[userID] = &newEConns
				delete(b.connID2UserIDMap, conn.GetID())

				// delete the key of userID when the length of the related
				// eventConn slice is 0.
				if len(newEConns) == 0 {
					delete(b.userID2EventConnMap, userID)
				}

				return nil
			}
		}

		return fmt.Errorf("can't find the conn of ID: %s", conn.GetID())
	}

	return fmt.Errorf("can't find the eventConns by userID: %s", userID)
}

// FindConn trys to find Conn by ID.
func (b *binder) FindConn(connID string) (*Conn, bool) {
	if connID == "" {
		return nil, false
	}

	userID, ok := b.connID2UserIDMap[connID]
	// if userID been found by connID, then find the Conn using userID
	if ok {
		if eConns, ok := b.userID2EventConnMap[userID]; ok {
			for i := range *eConns {
				if (*eConns)[i].Conn.GetID() == connID {
					return (*eConns)[i].Conn, true
				}
			}
		}

		return nil, false
	}

	// userID not found, iterate all the conns
	for _, eConns := range b.userID2EventConnMap {
		for i := range *eConns {
			if (*eConns)[i].Conn.GetID() == connID {
				return (*eConns)[i].Conn, true
			}
		}
	}

	return nil, false
}

// FilterConn searches the conns related to userID, and filtered by
// event. The userID can't be empty. The event will be ignored if it's empty.
// All the conns related to the userID will be returned if the event is empty.
func (b *binder) FilterConn(userID, event string) ([]*Conn, error) {
	if userID == "" {
		return nil, errors.New("userID can't be empty")
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	if eConns, ok := b.userID2EventConnMap[userID]; ok {
		ecs := make([]*Conn, 0, len(*eConns))
		for i := range *eConns {
			if event == "" || (*eConns)[i].Event == event {
				ecs = append(ecs, (*eConns)[i].Conn)
			}
		}
		return ecs, nil
	}

	return []*Conn{}, nil
}
