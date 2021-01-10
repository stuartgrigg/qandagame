package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/stuartgrigg/qandagame/engine"
	"github.com/stuartgrigg/qandagame/logging"
	"github.com/stuartgrigg/qandagame/server"
)

func main() {
	address := os.Getenv("SERVER_ADDRESS")

	logger := logging.NewLogger("logs.txt")
	defer logger.Close()

	fmt.Println("Starting worker")
	w, updates := engine.NewWorker(logger)
	go w.Run()

	fmt.Println("Starting server")
	s := server.NewServer(w, updates)
	go s.GameUpdateBroadcaster()
	http.HandleFunc("/ws", s.WebSocketHandler)
	http.HandleFunc("/", s.RootHandler)
	if err := http.ListenAndServe(address, nil); err != nil {
		log.Fatal(err)
	}
}
