package wserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// websocketHandler defines to handle websocket upgrade request.
type websocketHandler struct {
	// upgrader is used to upgrade request.
	upgrader *websocket.Upgrader

	// cm stores relations about websocket connection and userID.
	cm *CommManager

	// calcUserIDFunc defines to calculate userID by token. The userID will
	// be equal to token if this function is nil.
	calcUserIDFunc func(token string) (userID string, ok bool)
}

// RegisterMessage defines message struct client send after connect
// to the server.
type RegisterMessage struct {
	Token string `json:"token"`
	Event string `json:"event"`
}

type lookupHandler struct {
	cm *CommManager
}

// if the channel is established in this server.
// if not, return 404 respond
// otherwise, 200 OK
func (lh *lookupHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	vars := r.URL.Query()
	userID := vars.Get("userid")

	if userID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(ErrRequestIllegal.Error()))
		return
	}

	found, _ := lh.cm.hasUser(userID)

	if found {
		w.WriteHeader(http.StatusNotFound)
	}

}

// First try to upgrade connection to websocket. If success, connection will
// be kept until client send close message or server drop them.
func (wh *websocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	wsConn, err := wh.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer wsConn.Close()

	// handle Websocket request
	conn := NewConn(wsConn, wh)

	conn.BeforeCloseFunc = func() {
		// unbind
		wh.cm.Unbind(conn)
	}

	conn.Listen()
}

// closeConns unbind conns filtered by userID and event and close them.
// The userID can't be empty, but event can be empty. The event will be ignored
// if empty.
func (wh *websocketHandler) closeConns(userID, event string) (int, error) {
	return 0, nil
}

// ErrRequestIllegal describes error when data of the request is unaccepted.
var ErrRequestIllegal = errors.New("request data illegal")

// pushHandler defines to handle push message request.
type pushHandler struct {
	// authFunc defines to authorize request. The request will proceed only
	// when it returns true.
	authFunc func(r *http.Request) bool
	cm       *CommManager
}

// Authorize if needed. Then decode the request and push message to each
// related websocket connection.
func (s *pushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// authorize
	if s.authFunc != nil {
		if ok := s.authFunc(r); !ok {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	// read request
	var msg CommMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&msg); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(ErrRequestIllegal.Error()))
		return
	}

	// validate the data
	if msg.UserID == "" || msg.CommID == "" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(ErrRequestIllegal.Error()))
		return
	}

	var obj *CommObject
	var err error

	defer s.cm.removeCommand(msg.UserID, msg.CommID)

	obj, err = s.push(msg.UserID, msg.CommID, msg.Message)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	d, _ := time.ParseDuration("1s")
	err = s.wait(obj, d)

	// timeout
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	result := strings.NewReader(obj.response.Msg)
	io.Copy(w, result)
}

// wait until the client give the response
// push a command, wait or not
// if wait, got a channel and wait on it
// if not, return after the command is successfully pushed
//
func (s *pushHandler) wait(obj *CommObject, timeout time.Duration) error {

	if obj == nil {
		return errors.New("command object cannot be empty")
	}

	select {
	case <-obj.waitCH:
		return nil
	case <-time.After(timeout):
		return errors.New("timeout waiting command response")
	}

}

func (s *pushHandler) push(userID, commID, message string) (*CommObject, error) {

	if userID == "" || commID == "" || message == "" {
		return nil, errors.New("parameters(userId, event, message) can't be empty")
	}

	var obj *CommObject
	var ok error
	if obj, ok = s.cm.newCommand(userID, commID); ok != nil {
		return nil, errors.New("create new command failed")
	}

	request := CommRequest{
		Id:  commID,
		Msg: message,
	}
	obj.request = &request
	obj.id = commID

	// filter connections by userID and event, then push message
	conn := obj.conn
	raw, _ := json.Marshal(&request)

	_, err := conn.Write(raw)

	if err != nil {
		return nil, err
	}

	return obj, nil
}

// PushMessage defines message struct send by client to push to each connected
// websocket client.
type PushMessage struct {
	UserID  string `json:"userId"`
	Event   string
	Message string
}

type CommMessage struct {
	UserID  string `json:"userId"`
	CommID  string `json:"commId"`
	Message string `json:"message"`
}
