package services

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type Device struct {
	Name            string    `json:"name"`
	Os              string    `json:"os"`
	Address         string    `json:"address"`
	Online          bool      `json:"online"`
	LastSeen        time.Time `json:"lastSeen"`
	UpdateAvailable bool      `json:"updateAvailable"`
}

func tailscaleDevices(c *Config) ([]Device, error) {
	var res struct {
		Devices []struct {
			Name               string    `json:"name"`
			Hostname           string    `json:"hostname"`
			Os                 string    `json:"os"`
			Addresses          []string  `json:"addresses"`
			ConnectedToControl bool      `json:"connectedToControl"`
			LastSeen           time.Time `json:"lastSeen"`
			UpdateAvailable    bool      `json:"updateAvailable"`
		} `json:"devices"`
	}
	// connectedToControl only comes with fields=all. "-" is the tailnet of the key
	err := call(client, "GET", "https://api.tailscale.com/api/v2/tailnet/-/devices?fields=all", map[string]string{"Authorization": "Bearer " + c.Secret}, nil, &res)
	if err != nil {
		return nil, fmt.Errorf("tailscale: %w", err)
	}

	devices := []Device{}
	for _, d := range res.Devices {
		// name is the magicdns one (box.tail1234.ts.net), the first part is what you named it
		name := strings.Split(d.Name, ".")[0]
		if name == "" {
			name = d.Hostname
		}
		device := Device{Name: name, Os: d.Os, Online: d.ConnectedToControl, LastSeen: d.LastSeen, UpdateAvailable: d.UpdateAvailable}
		if len(d.Addresses) > 0 {
			device.Address = d.Addresses[0]
		}
		devices = append(devices, device)
	}

	sort.Slice(devices, func(i, j int) bool {
		if devices[i].Online != devices[j].Online {
			return devices[i].Online
		}
		return devices[i].Name < devices[j].Name
	})
	return devices, nil
}

func checkTailscale(c Config) error {
	_, err := tailscaleDevices(&c)
	return err
}

func TailscaleDevices() ([]Device, error) {
	c, err := loadEnabled("tailscale")
	if err != nil {
		return nil, err
	}
	return tailscaleDevices(c)
}
