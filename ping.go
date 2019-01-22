package dc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dennwc/go-dcpp/nmdc"
)

// Ping fetches the information about the specified hub.
func Ping(ctx context.Context, addr string) (*HubInfo, error) {
	// probe first, if protocol is not specified
	i := strings.Index(addr, "://")
	if i < 0 {
		s, err := Probe(ctx, addr)
		if err != nil {
			return nil, err
		}
		addr = s
		i = strings.Index(addr, "://")
	}

	switch addr[:i+3] {
	case nmdcSchema, nmdcsSchema:
		hub, err := nmdc.Ping(ctx, addr)
		if err != nil {
			return nil, err
		}
		info := &HubInfo{
			Name: hub.Name,
			Desc: hub.Topic,
			Addr: []string{addr},
			Server: &Software{
				Name: hub.Server.Name,
				Vers: hub.Server.Vers,
				Ext:  hub.Ext,
			},
			Users: make([]HubUser, 0, len(hub.Users)),
		}
		for _, u := range hub.Users {
			info.Users = append(info.Users, HubUser{
				Name:  string(u.Name),
				Share: u.ShareSize,
				Client: &Software{
					Name: u.Client,
					Vers: u.Version,
				},
			})
		}
		return info, nil
	default:
		// FIXME: ADC
		return nil, fmt.Errorf("unsupported protocol: %q", addr)
	}
}

type HubInfo struct {
	Name   string        `json:"name"`
	Desc   string        `json:"desc"`
	Server *Software     `json:"server"`
	Addr   []string      `json:"addr"`
	Uptime time.Duration `json:"uptime"`
	Users  []HubUser     `json:"users"`
}

type HubUser struct {
	Name   string    `json:"name"`
	Client *Software `json:"client"`
	Share  uint64    `json:"share"`
}

// Software version.
type Software struct {
	Name string   `json:"name"`
	Vers string   `json:"vers"`
	Ext  []string `json:"ext"`
}