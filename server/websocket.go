package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func (s *Server) WebSocketHandler(w http.ResponseWriter, r *http.Request) {

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Websocket upgrade error: %v\n", err)
	}

	// helpful log statement to show connections
	log.Println("Websocket Client Connected")

	s.wsClients[ws] = r.URL.Query()["wstrack"][0]

	// reader spins forever
	s.reader(ws)
}

func (s *Server) GameUpdateBroadcaster() {
	for update := range s.updatesChannel {
		fmt.Println("update received")
		// Sleep so the response is sent first
		for conn, ip := range s.wsClients {
			// Don't send an update to whoever triggered it
			if update.ClientID == ip {
				continue
			}
			err := conn.WriteMessage(websocket.TextMessage, []byte{})
			if err != nil {
				conn.Close()
				delete(s.wsClients, conn)
			}
		}
	}
}

func (s *Server) reader(conn *websocket.Conn) {
	for {
		// read in a message
		messageType, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Websocket read error: %v\n", err)
			conn.Close()
			delete(s.wsClients, conn)
			return
		}
		if messageType == websocket.CloseMessage {
			delete(s.wsClients, conn)
			return
		}

	}
}
