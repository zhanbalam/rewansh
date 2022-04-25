package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

type wanEventType uint8

const (
	eventWanDown wanEventType = iota + 1
	eventWanUp
	eventWanStable
)

type wanEvent struct {
	wanPri int
	event  wanEventType
}

type wanWrap struct {
	wan     *wan
	active  bool
	command []string
}

type wanManager struct {
	ping      *ping
	wans      []*wanWrap
	minUptime uint
	done      chan bool

	log *log.Logger
}

func newWanManager(c *config, l *log.Logger) *wanManager {
	wman := &wanManager{
		ping:      &c.Ping,
		log:       l,
		minUptime: c.MinUptime,
		done:      make(chan bool, 1),
	}
	for i, s := range c.Servers {
		wman.wans = append(wman.wans, &wanWrap{
			wan:     newWan(i+1, s.WsURL, l),
			active:  s.IsActive,
			command: s.Command,
		})
	}
	return wman
}

func (wm *wanManager) run() {
	wanCh := make(chan wanEvent)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, w := range wm.wans {
		if w.active {
			go w.wan.monitor(ctx, wm.ping.Active, wm.ping.PongWait, wm.minUptime, wanCh)
		} else {
			go w.wan.monitor(ctx, wm.ping.Idle, wm.ping.PongWait, wm.minUptime, wanCh)
		}
	}
	go wm.listen(ctx, wanCh)
	<-wm.done
}

func (wm *wanManager) listen(ctx context.Context, ch <-chan wanEvent) {
	for {
		select {
		case e := <-ch:
			switch e.event {
			case eventWanDown:
				activePri := wm.getActiveWanPriority()
				if e.wanPri == activePri {
					wm.log.Printf("active wan is down with priority %d", e.wanPri)
					wm.log.Printf("switching to another wan...")
					if err := wm.switchActiveWan(); err != nil {
						wm.log.Errorf("cannot switch wan: %s", err)
					}
				} else {
					wm.log.Printf("idle wan is down with priority %d", e.wanPri)
					// Get first stable wan and check whether it's priority higher than the active wan's
					stable := wm.getStableIdleWan()
					if stable != nil && stable.wan.pri < activePri {
						alive := wm.getAliveIdleWan()
						if alive != nil && alive.wan.pri < stable.wan.pri {
							continue
						}
						// if no alive wan with priority higher than this stable left -> trigger switch
						wm.log.Printf("switching to a stable wan with a higher priority %d", stable.wan.pri)
						if err := wm.switchActiveWan(); err != nil {
							wm.log.Errorf("cannot switch wan: %s", err)
						}
					}
				}
			case eventWanUp:
				wm.log.Printf("wan is up with priority %d", e.wanPri)
				if w := wm.getActiveWan(); w == nil {
					wm.log.Printf("switching to alive wan with priority %d", e.wanPri)
					if err := wm.switchActiveWan(); err != nil {
						wm.log.Errorf("cannot switch wan: %s", err)
					}
				}
			case eventWanStable:
				wm.log.Printf("wan is stable with priority %d", e.wanPri)
				if e.wanPri < wm.getActiveWanPriority() {
					alive := wm.getAliveIdleWan()
					if alive != nil && alive.wan.pri < e.wanPri {
						// if there is an idle alive wan with higher priority waiting to be stable - wait for it
						continue
					}
					wm.log.Printf("switching to a stable wan with a higher priority %d", e.wanPri)
					if err := wm.switchActiveWan(); err != nil {
						wm.log.Errorf("cannot switch wan: %s", err)
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (wm *wanManager) switchActiveWan() error {
	from := wm.getActiveWan()
	if from != nil {
		from.active = false
	}
	to := wm.getStableIdleWan()
	if to == nil {
		to = wm.getAliveIdleWan()
	}
	if to == nil {
		return fmt.Errorf("no stable or alive wan available")
	}
	wm.log.Debugf("executing command: %s", strings.Join(to.command, " "))
	output, err := wm.execute(to.command)
	if err != nil {
		return fmt.Errorf("command execution failed: %s", err)
	}
	wm.log.Debugf("command execution complete: %s", output)
	to.active = true
	wm.log.Printf("switched to wan with priority %d", to.wan.pri)
	return nil
}

func (wm *wanManager) getActiveWan() *wanWrap {
	for _, w := range wm.wans {
		if w.active {
			return w
		}
	}
	return nil
}

func (wm *wanManager) getActiveWanPriority() int {
	if w := wm.getActiveWan(); w != nil {
		return w.wan.pri
	}
	return len(wm.wans) + 1
}

func (wm *wanManager) getStableIdleWan() *wanWrap {
	for _, w := range wm.wans {
		if !w.active && w.wan.isStable() {
			return w
		}
	}
	return nil
}

func (wm *wanManager) getAliveIdleWan() *wanWrap {
	for _, w := range wm.wans {
		if !w.active && w.wan.isAlive() {
			return w
		}
	}
	return nil
}

func (wm *wanManager) execute(cmd []string) (string, error) {
	c := exec.Command(cmd[0], cmd[1:]...)
	out, err := c.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func (wm *wanManager) stop() {
	// if err := dumpState(); err != nil {
	// 	wm.log.Errorf("Could not store wans state: %s", err)
	// }
	wm.done <- true
}

// func (wm *wanManager) dumpState() error {
// }

// func (wm *wanManager) loadState() error {
// }
