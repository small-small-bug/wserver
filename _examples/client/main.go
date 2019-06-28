package main

import (
	"../../../wserver"
	"encoding/json"
	"flag"
	"github.com/gorilla/websocket"
	"log"
	"strconv"
	"sync"
)

func main() {

	var url = flag.String("url", "ws://127.0.0.1:12345/ws", "websocket server: ws://address:port/path")
	var concurrency = flag.Int("number", 10, "the number of concurrent clients")

	flag.Parse()

	done := make(chan struct{})

	// get ^C from the terminal
	/*go func(){
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		log.Println("exiting the program...")
		os.Exit(1)
	}()
	*/
	var wg sync.WaitGroup
	wg.Add(*concurrency)

	users := make([]string, *concurrency)

	for i := range users {
		users[i] = strconv.Itoa(i)
		//start the client
		go func(user string) {
			defer wg.Done()
			newEchoClient(*url, user, done)
		}(users[i])
	}
	// wait for termination
	wg.Wait()
}

func newEchoClient(url, user string, done chan struct{}) error {
	c, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("dial:", err)
		return err
	}
	log.Println("start to run client")
	defer c.Close()

	rm := wserver.RegisterMessage{
		Token: user,
		Event: "what ever",
	}
	strRm, _ := json.Marshal(rm)
	msg := wserver.WSMessage{
		Kind: wserver.RegisterMessageType,
		Body: string(strRm),
	}

	strMsg, _ := json.Marshal(msg)

	err = c.WriteMessage(websocket.TextMessage, []byte(strMsg))

	if err != nil {
		log.Println("write:", err)
		c.Close()
		return err
	}
	log.Println("register succeed")

	// wait until the server close the connection
readloop:
	for {
		select {
		case <-done:
			break readloop
		default:

			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return err
			}
			log.Printf("recv: %s", message)
			wsm := wserver.WSMessage{
				Kind: wserver.NormalMessageType,
				Body: string(message),
			}

			strMsg, _ := json.Marshal(wsm)
			err = c.WriteMessage(websocket.TextMessage, []byte(strMsg))
			if err != nil {
				return err
			}

		}
	}

	return nil

}
