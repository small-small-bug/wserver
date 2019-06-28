package wserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func Test_Server_1(t *testing.T) {
	port := 12345
	userID := uuid.New().String()

	count := 100

	s := NewServer(":" + strconv.Itoa(port))

	// run wserver
	go runBasicWServer(s)
	time.Sleep(time.Millisecond * 300)

	// push message
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Second)
		log.Println("start to push ...")
		pushURL := "http://127.0.0.1:12345/push"
		contentType := "application/json"

		for i := 0; i < count; i++ {
			pm := CommMessage{
				UserID:  userID,
				CommID:  uuid.New().String(),
				Message: fmt.Sprintf("Hello in %d", i),
			}
			b, _ := json.Marshal(pm)

			resp, _ := http.DefaultClient.Post(pushURL, contentType, bytes.NewReader(b))

			body, _ := ioutil.ReadAll(resp.Body)
			fmt.Println(string(body))

			resp.Body.Close()

			time.Sleep(time.Microsecond)
		}

		close(done)
	}()

	// dial websocket
	log.Println("start to connect ...")
	url := fmt.Sprintf("ws://127.0.0.1:%d/ws", port)

	_ = runBasicClient(userID, url, done)

}

func runBasicWServer(s *Server) {
	if err := s.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func runBasicClient(userID, server string, done chan struct{}) error {

	c, _, err := websocket.DefaultDialer.Dial(server, nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	log.Println("start to run client")
	defer c.Close()

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
			wsm := WSMessage{
				Kind: NormalMessageType,
				Body: string(message),
			}

			strMsg, _ := json.Marshal(wsm)
			err = c.WriteMessage(websocket.TextMessage, []byte(strMsg))
			if err != nil {
				return
			}
		}
	}()

	rm := RegisterMessage{
		Token: userID,
		Event: "what ever",
	}
	strRm, _ := json.Marshal(rm)
	msg := WSMessage{
		Kind: RegisterMessageType,
		Body: string(strRm),
	}

	strMsg, _ := json.Marshal(msg)

	err = c.WriteMessage(websocket.TextMessage, []byte(strMsg))

	if err != nil {
		log.Println("write:", err)
		return err
	}
	log.Println("register succeed")

	// wait until the server close the connection
	for {
		select {
		case <-done:
			return nil
		}
	}
}
