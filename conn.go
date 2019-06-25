package wserver

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"sync"
	"time"
)

const (
	RegisterMessageType = 1
	NormalMessageType   = 255
)

type WSMessage struct {
	Kind int    `json:"Kind"`
	Body string `json:"Body"`
}

// Conn wraps websocket.Conn with Conn. It defines to listen and read
// data from Conn.
type Conn struct {
	Conn *websocket.Conn

	AfterReadFunc   func(messageType int, r io.Reader)
	BeforeCloseFunc func()

	once   sync.Once
	id     string
	stopCh chan struct{}

	// if a socket is bound, then the string userId must not be empty
	userId *string

	// if the socket registered or not
	registered bool

	// the websocket handler
	// must not be empty
	wh *websocketHandler
}

// Write write p to the websocket connection. The error returned will always
// be nil if success.
func (c *Conn) Write(p []byte) (n int, err error) {
	select {
	case <-c.stopCh:
		return 0, errors.New("Conn is closed, can't be written")
	default:
		err = c.Conn.WriteMessage(websocket.TextMessage, p)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	}
}

// GetID returns the Id generated using UUID algorithm.
func (c *Conn) GetID() string {
	c.once.Do(func() {
		u := uuid.New()
		c.id = u.String()
	})

	return c.id
}

func (c *Conn) OnMessage(messageType int, r io.Reader) {
	var wm WSMessage
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&wm); err != nil {
		return
	}

	if wm.Kind == RegisterMessageType {
		// TODO close socket in this case
		// should exit goroutine
		c.HandleRegister(wm.Body)
		return
	} else if wm.Kind == NormalMessageType {
		c.HandleCommand(wm.Body)
		return
	}

}

func (c *Conn) HandleRegister(body string) error {

	rm := RegisterMessage{}
	err := json.Unmarshal([]byte(body), &rm)

	if err != nil {
		return err
	}

	wh := c.wh

	userID := rm.Token
	if wh.calcUserIDFunc != nil {
		uID, ok := wh.calcUserIDFunc(rm.Token)
		if !ok {
			return errors.New("calcUserIDFunc failed")
		}
		userID = uID
	}

	// bind
	return wh.cm.Bind(userID, c)

}

func (c *Conn) HandleCommand(body string) error {

	cr := CommResponse{}

	err := json.Unmarshal([]byte(body), &cr)

	if err != nil {
		return err
	}

	wh := c.wh
	commandID := cr.Id

	userID := c.userId
	if userID == nil {
		return errors.New("this connection is not registered yet")
	}

	obj, _ := wh.cm.lookupCommand(*userID, commandID)

	if obj == nil {
		return errors.New("cannot find this command")
	}

	obj.response = &cr
	close(obj.waitCH)

	return nil
}

// Listen listens for receive data from websocket connection. It blocks
// until websocket connection is closed.
func (c *Conn) Listen() {
	c.Conn.SetCloseHandler(func(code int, text string) error {
		if c.BeforeCloseFunc != nil {
			c.BeforeCloseFunc()
		}

		if err := c.Close(); err != nil {
			log.Println(err)
		}

		message := websocket.FormatCloseMessage(code, "")
		err := c.Conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
		return err
	})

	// Keeps reading from Conn util get error.
ReadLoop:
	for {
		select {
		case <-c.stopCh:
			break ReadLoop
		default:
			messageType, r, err := c.Conn.NextReader()
			if err != nil {
				// TODO: handle read error maybe
				break ReadLoop
			}
			// TODO handle error
			c.OnMessage(messageType, r)

		}
	}
}

// Close close the connection.
func (c *Conn) Close() error {
	select {
	case <-c.stopCh:
		return errors.New("Conn already been closed")
	default:
		c.Conn.Close()
		close(c.stopCh)
		return nil
	}
}

// NewConn wraps conn.
func NewConn(conn *websocket.Conn, wh *websocketHandler) *Conn {
	return &Conn{
		wh:     wh,
		Conn:   conn,
		stopCh: make(chan struct{}),
	}
}
