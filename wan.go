package main

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type connEventType uint8

const (
	eventConnDown connEventType = iota + 1
	eventConnUp
)

type wanState uint8

const (
	stateInit wanState = iota
	stateDead
	stateAlive
	stateStable
)

type stateWrap struct {
	state   wanState
	stateAt time.Time
	mu      sync.RWMutex
}

type wan struct {
	pri   int
	ws    *wsclient
	state stateWrap

	log *log.Logger
}

func newWan(pri int, wsUrl string, l *log.Logger) *wan {
	return &wan{
		pri: pri,
		ws:  newWs(wsUrl, l),
		log: l,
	}
}

func (s *stateWrap) is(state wanState) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == state
}

func (s *stateWrap) set(state wanState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
	s.stateAt = time.Now()
}

func (w *wan) isAlive() bool {
	return w.state.is(stateAlive)
}

func (w *wan) isStable() bool {
	return w.state.is(stateStable)
}

func (w *wan) monitor(ctx context.Context, pingPeriod, pongWait, minUptime uint, wmanCh chan wanEvent) {
	go w.ws.keepAlive(ctx, pingPeriod, pongWait)
	for {
		select {
		case ev := <-w.ws.events:
			switch ev {
			case eventConnDown:
				if !w.state.is(stateDead) {
					w.log.Debugf("connection is down: %s", w.ws.url)
					w.state.set(stateDead)
					wmanCh <- wanEvent{
						wanPri: w.pri,
						event:  eventWanDown,
					}
				}
			case eventConnUp:
				w.log.Debugf("connection is up: %s", w.ws.url)
				w.state.set(stateAlive)
				wmanCh <- wanEvent{
					wanPri: w.pri,
					event:  eventWanUp,
				}
			}
		case <-time.After(time.Duration(minUptime) * time.Second):
			if w.state.is(stateAlive) {
				w.state.set(stateStable)
				w.log.Debugf("minuptime for %s", w.ws.url)
				wmanCh <- wanEvent{
					wanPri: w.pri,
					event:  eventWanStable,
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
