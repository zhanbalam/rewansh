package main

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

type wsclient struct {
	url    string
	events chan connEventType

	conn *websocket.Conn
	mu   sync.Mutex

	log *log.Logger
}

func newWs(addr string, l *log.Logger) *wsclient {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/ws"}
	return &wsclient{
		url:    u.String(),
		events: make(chan connEventType),
		log:    l,
	}
}

func (ws *wsclient) connect(ctx context.Context) (conn *websocket.Conn) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.conn != nil {
		return ws.conn
	}
	var err error
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for ; ; <-ticker.C {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, _, err = websocket.DefaultDialer.Dial(ws.url, nil)
			if err != nil {
				ws.log.Debugf("cannot connect to %s, error: %s", ws.url, err)
				ws.events <- eventConnDown
				continue
			}
			ws.log.Debugf("connected to %s", ws.url)
			ws.conn = conn
			ws.events <- eventConnUp
			return conn
		}
	}
}

func (ws *wsclient) close() {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	ws.conn.Close()
	ws.conn = nil
	ws.log.Debugf("disconnected from %s", ws.url)
	ws.events <- eventConnDown
}

func (ws *wsclient) listen(ctx context.Context, pongWait uint) {
	ws.log.Debugf("listening for the messages: %s", ws.url)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			conn := ws.connect(ctx)
			if conn == nil {
				continue
			}
			conn.SetPongHandler(func(string) error {
				ws.log.Debugf("got pong from: %s", ws.url)
				conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
				return nil
			})
			_, _, err := conn.ReadMessage()
			if err != nil {
				ws.log.Debugf("read error: %s", err)
				ws.close()
				continue
			}
		}
	}
}

func (ws *wsclient) keepAlive(ctx context.Context, pingPeriod, pongWait uint) {
	go ws.listen(ctx, pongWait)
	ws.log.Debugf("ping pong started for %s", ws.url)
	ticker := time.NewTicker(time.Duration(pingPeriod) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			conn := ws.connect(ctx)
			if conn == nil {
				continue
			}
			ws.log.Debugf("sending ping to: %s", ws.url)
			conn.SetReadDeadline(time.Now().Add(time.Duration(pongWait) * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				ws.log.Debugf("write error: %s", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
