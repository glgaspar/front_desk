package controller

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/glgaspar/front_desk/features/apps"
	"github.com/glgaspar/front_desk/features/integrations"
	"github.com/glgaspar/front_desk/features/integrations/npm"
	"github.com/glgaspar/front_desk/features/integrations/services"
	"github.com/glgaspar/front_desk/features/monitoring"
	"github.com/labstack/echo/v4"
)

func GetDiskForecast(c echo.Context) error {
	forecast, err := monitoring.GetDiskForecast()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: forecast})
}

func GetSeerrPending(c echo.Context) error {
	pending, err := services.SeerrPendingRequests()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d requests waiting", len(pending)), Data: pending})
}

func AnswerSeerrRequest(c echo.Context) error {
	id, err := strconv.Atoi(c.Param("id"))
	action := c.Param("action")
	if err != nil || (action != "approve" && action != "decline") {
		return c.JSON(http.StatusBadRequest, Response{Status: false, Message: "Both id (int) and action (\"approve\", \"decline\") must be sent"})
	}

	err = services.SeerrAnswer(id, action)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful"})
}

func GetMediaCalendar(c echo.Context) error {
	days, err := strconv.Atoi(c.QueryParam("days"))
	if err != nil || days < 1 || days > 31 {
		days = 7
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: services.MediaCalendar(days)})
}

func GetPiholeTop(c echo.Context) error {
	top, err := services.GetPiholeTop()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, Response{Status: false, Message: err.Error()})
	}

	return c.JSON(http.StatusOK, Response{Status: true, Message: "Operation successful", Data: top})
}

type AttentionItem struct {
	Severity string `json:"severity"` // critical | warning
	Kind     string `json:"kind"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Link     string `json:"link"`
}

// everything that needs a human, from every place front desk knows about. each check runs on its own
// and an integration that doesnt answer becomes an item too, instead of breaking the list
func GetAttention(c echo.Context) error {
	items := []AttentionItem{}
	var mu sync.Mutex
	var wg sync.WaitGroup

	add := func(item AttentionItem) {
		mu.Lock()
		items = append(items, item)
		mu.Unlock()
	}
	check := func(integration string, run func() error) {
		if integration != "" {
			if enabled, _ := integrations.CheckFor(integration); !enabled {
				return
			}
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run(); err != nil {
				add(AttentionItem{Severity: "warning", Kind: "integration", Title: integration + " is not answering", Detail: err.Error(), Link: "/config"})
			}
		}()
	}

	check("", func() error {
		appList, err := new(apps.App).GetList()
		if err != nil {
			return err
		}
		for _, a := range appList {
			if a.Broken() {
				add(AttentionItem{Severity: "critical", Kind: "container", Title: displayName(a) + " is " + a.State.Status, Detail: strings.TrimSpace(fmt.Sprintf("exit code %d %s", a.State.ExitCode, a.State.Error)), Link: "/"})
			}
		}
		return nil
	})

	check("", func() error {
		uptime, err := monitoring.GetUptime("24h")
		if err != nil {
			return err
		}
		for _, u := range uptime {
			if !u.Up {
				add(AttentionItem{Severity: "critical", Kind: "down", Title: u.AppName + " is not answering", Detail: "down since " + u.ChangedAt.Format("02/01 15:04"), Link: "/monitoring"})
			}
		}
		return nil
	})

	check("", func() error {
		updates, err := apps.GetAllUpdates(false)
		if err != nil {
			return err
		}
		var refs []string
		for ref, u := range updates {
			if u.Status == "update_available" {
				refs = append(refs, ref)
			}
		}
		if len(refs) > 0 {
			sort.Strings(refs)
			add(AttentionItem{Severity: "warning", Kind: "updates", Title: fmt.Sprintf("%d updates available", len(refs)), Detail: strings.Join(refs, ", "), Link: "/"})
		}
		return nil
	})

	check("", func() error {
		disks, err := monitoring.GetDiskForecast()
		if err != nil {
			return err
		}
		for _, d := range disks {
			switch {
			case d.UsedPercent >= 95 || (d.DaysToFull != nil && *d.DaysToFull < 7):
				add(AttentionItem{Severity: "critical", Kind: "disk", Title: d.Path + " is almost full", Detail: diskDetail(d), Link: "/monitoring"})
			case d.UsedPercent >= 85 || (d.DaysToFull != nil && *d.DaysToFull < 30):
				add(AttentionItem{Severity: "warning", Kind: "disk", Title: d.Path + " is filling up", Detail: diskDetail(d), Link: "/monitoring"})
			}
		}
		return nil
	})

	check("nginxproxymanager", func() error {
		certs, err := npm.ListCertificates()
		if err != nil {
			return err
		}
		for _, cert := range certs {
			expires, err := time.Parse("2006-01-02 15:04:05", cert.ExpiresOn)
			if err != nil {
				expires, err = time.Parse(time.RFC3339, cert.ExpiresOn)
			}
			if err != nil {
				continue
			}
			days := time.Until(expires).Hours() / 24
			name := cert.NiceName
			if name == "" {
				name = strings.Join(cert.DomainNames, ", ")
			}
			if days < 3 {
				add(AttentionItem{Severity: "critical", Kind: "certificate", Title: name + " certificate expires " + inDays(days), Detail: "npm did not renew it", Link: "/proxy"})
			} else if days < 14 {
				add(AttentionItem{Severity: "warning", Kind: "certificate", Title: name + " certificate expires " + inDays(days), Detail: "npm should renew it on its own, check if it did", Link: "/proxy"})
			}
		}
		return nil
	})

	check("seerr", func() error {
		pending, err := services.SeerrPendingRequests()
		if err != nil {
			return err
		}
		if len(pending) > 0 {
			var titles []string
			for _, p := range pending {
				titles = append(titles, p.Title)
			}
			add(AttentionItem{Severity: "warning", Kind: "seerr", Title: fmt.Sprintf("%d requests waiting for approval", len(pending)), Detail: strings.Join(titles, ", "), Link: "/widgets"})
		}
		return nil
	})

	check("tdarr", func() error {
		stats, err := services.GetTdarrStats()
		if err != nil {
			return err
		}
		if stats.Errored > 0 {
			add(AttentionItem{Severity: "warning", Kind: "tdarr", Title: fmt.Sprintf("tdarr has %d files with errors", stats.Errored), Link: "/widgets"})
		}
		return nil
	})

	check("tailscale", func() error {
		devices, err := services.TailscaleDevices()
		if err != nil {
			return err
		}
		for _, d := range devices {
			if d.Expires.IsZero() {
				continue
			}
			days := time.Until(d.Expires).Hours() / 24
			if days < 14 {
				severity := "warning"
				if days < 3 {
					severity = "critical"
				}
				add(AttentionItem{Severity: severity, Kind: "tailscale", Title: d.Name + " tailscale key expires " + inDays(days), Detail: "renew it on the tailscale admin page or disable key expiry", Link: "/widgets"})
			}
		}
		return nil
	})

	wg.Wait()

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Severity == "critical" && items[j].Severity != "critical"
	})

	return c.JSON(http.StatusOK, Response{Status: true, Message: fmt.Sprintf("%d things need attention", len(items)), Data: items})
}

func displayName(a apps.App) string {
	if a.Name != "" {
		return a.Name
	}
	return a.ImageRef
}

func inDays(days float64) string {
	switch {
	case days < 0:
		return "already expired"
	case days < 1:
		return "today"
	case days < 2:
		return "tomorrow"
	}
	return fmt.Sprintf("in %.0f days", days)
}

func diskDetail(d monitoring.DiskForecast) string {
	detail := fmt.Sprintf("%.0f%% used", d.UsedPercent)
	if d.DaysToFull != nil {
		detail += fmt.Sprintf(", full in ~%.0f days at this rate", *d.DaysToFull)
	}
	return detail
}
