package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	defaultPingActive = 10
	defaultPingIdle   = 60
	defaultPongWait   = 5
	defaultMinUptime  = 900
)

type server struct {
	WsURL    string   `yaml:"ws_url"`
	IsActive bool     `yaml:"is_active"`
	Command  []string `yaml:"command"`
}

type ping struct {
	Active   uint `yaml:"active"`
	Idle     uint `yaml:"idle"`
	PongWait uint `yaml:"pong_wait"`
}

type config struct {
	Ping      ping      `yaml:"ping"`
	MinUptime uint      `yaml:"min_uptime"`
	Servers   []*server `yaml:"servers"`
}

func loadConfig(path string) (*config, error) {
	c := &config{}
	c.setDefaults()

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open config file: %v", err)
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err := d.Decode(&c); err != nil {
		return nil, fmt.Errorf("Failed read config file: %v", err)
	}

	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("Validation failed: %v", err)
	}

	return c, nil
}

func (c *config) validate() error {
	if len(c.Servers) < 2 {
		return fmt.Errorf("failover requires at least 2 wans")
	}
	var activeDefined bool
	for i, s := range c.Servers {
		if len(s.Command) == 0 {
			return fmt.Errorf("command is not defined for wan #%d", i+1)
		}
		if s.IsActive {
			activeDefined = true
		}
	}
	if activeDefined == false {
		return fmt.Errorf("active wan is not defined")
	}
	return nil
}

func (c *config) setDefaults() {
	c.Ping.Active = defaultPingActive
	c.Ping.Idle = defaultPingIdle
	c.Ping.PongWait = defaultPongWait
	c.MinUptime = defaultMinUptime
}
