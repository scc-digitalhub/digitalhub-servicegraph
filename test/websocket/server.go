// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package websocket

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}
var connections map[*websocket.Conn]bool
var mutex *sync.Mutex

func RunServer() {

	connections = make(map[*websocket.Conn]bool)
	mutex = &sync.Mutex{}

	serverMux := http.NewServeMux()
	serverMux.HandleFunc("/ws", wsHandler)

	wsSrv := &http.Server{
		Addr:         "127.0.0.1:9090",
		WriteTimeout: 10 * time.Second,
		ReadTimeout:  10 * time.Second,
		Handler:      serverMux,
	}
	err := wsSrv.ListenAndServe()
	if err != nil {
		fmt.Println(fmt.Errorf("error starting server: %w", err))
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(fmt.Errorf("error upgrading: %w", err))
		return
	}
	// defer conn.Close()

	mutex.Lock()
	connections[conn] = true
	mutex.Unlock()

	go handleConnection(conn)
}

func handleConnection(conn *websocket.Conn) {

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		fmt.Println("Echoing message")
		conn.WriteMessage(messageType, message)
	}
}
