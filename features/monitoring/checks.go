package monitoring

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/glgaspar/front_desk/connection"
	"github.com/glgaspar/front_desk/features/apps"
)

type CheckResult struct {
	AppKey          string
	AppName         string
	ContainerStatus string
	HttpStatus      *int
	LatencyMs       *int
	Up              bool
	Error           string
}

type State struct {
	Up       bool
	Failures int
}

type Event struct {
	Type    string    `json:"type"` // down | up | test
	AppKey  string    `json:"appKey"`
	AppName string    `json:"appName"`
	Error   string    `json:"error"`
	Time    time.Time `json:"time"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

// container ids change every rebuild, the dir doesnt
func appKey(app apps.App) string {
	if app.Dir == "" {
		return app.Name
	}
	return app.Dir + "|" + app.Name
}

func checkUrl(appUrl string) (int, int, error) {
	if !strings.Contains(appUrl, "://") {
		appUrl = "https://" + appUrl
	}
	start := time.Now()
	res, err := httpClient.Get(appUrl)
	latency := int(time.Since(start).Milliseconds())
	if err != nil {
		return 0, latency, err
	}
	res.Body.Close()
	return res.StatusCode, latency, nil
}

func checkApp(app apps.App) CheckResult {
	result := CheckResult{
		AppKey:          appKey(app),
		AppName:         app.Name,
		ContainerStatus: app.State.Status,
		Up:              app.State.Status == "running",
	}
	if !result.Up {
		result.Error = "container is " + app.State.Status
		return result
	}

	if strings.TrimSpace(app.Url) == "" {
		return result
	}

	status, latency, err := checkUrl(app.Url)
	result.LatencyMs = &latency
	if err != nil {
		result.Up = false
		result.Error = err.Error()
		return result
	}
	result.HttpStatus = &status
	// 401, 403 and friends still mean its alive
	if status >= 500 {
		result.Up = false
		result.Error = fmt.Sprintf("http %d", status)
	}
	return result
}

// only yells after a few failures in a row, and only once when it comes back
func evaluateTransition(prev *State, up bool, threshold int) (State, string) {
	state := State{Up: true}
	if prev != nil {
		state = *prev
	}

	if up {
		state.Failures = 0
		if !state.Up {
			state.Up = true
			return state, "up"
		}
		return state, ""
	}

	state.Failures++
	if state.Up && state.Failures >= threshold {
		state.Up = false
		return state, "down"
	}
	return state, ""
}

func RunChecks() error {
	appList, err := new(apps.App).GetList()
	if err != nil {
		return err
	}

	var results []CheckResult
	var lock sync.Mutex
	var wg sync.WaitGroup
	limit := make(chan struct{}, 5)

	for _, app := range appList {
		if app.Name == "" {
			continue
		}
		wg.Add(1)
		go func(app apps.App) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()

			result := checkApp(app)
			lock.Lock()
			results = append(results, result)
			lock.Unlock()
		}(app)
	}
	wg.Wait()

	conn, err := connection.Db()
	if err != nil {
		return err
	}

	threshold := envInt("MONITOR_FAIL_THRESHOLD", 2)
	for _, result := range results {
		_, err = conn.Exec(`
		insert into frontdesk.health_checks (app_key, app_name, container_status, http_status, latency_ms, up, error)
		values ($1, $2, $3, $4, $5, $6, nullif($7, ''))`,
			result.AppKey, result.AppName, result.ContainerStatus, result.HttpStatus, result.LatencyMs, result.Up, result.Error)
		if err != nil {
			return err
		}

		var prev *State
		var saved State
		err = conn.QueryRow(`select up, consecutive_failures from frontdesk.monitor_state where app_key = $1`, result.AppKey).Scan(&saved.Up, &saved.Failures)
		if err == nil {
			prev = &saved
		} else if err != sql.ErrNoRows {
			return err
		}

		state, event := evaluateTransition(prev, result.Up, threshold)
		_, err = conn.Exec(`
		insert into frontdesk.monitor_state (app_key, up, consecutive_failures, changed_at, last_checked_at)
		values ($1, $2, $3, now(), now())
		on conflict (app_key) do update set
			up = excluded.up,
			consecutive_failures = excluded.consecutive_failures,
			changed_at = case when frontdesk.monitor_state.up <> excluded.up then now() else frontdesk.monitor_state.changed_at end,
			last_checked_at = now()`,
			result.AppKey, state.Up, state.Failures)
		if err != nil {
			return err
		}

		if event != "" {
			Dispatch(Event{Type: event, AppKey: result.AppKey, AppName: result.AppName, Error: result.Error, Time: time.Now()})
		}
	}

	return nil
}

func Cleanup() error {
	conn, err := connection.Db()
	if err != nil {
		return err
	}

	days := envInt("MONITOR_RETENTION_DAYS", 30)
	_, err = conn.Exec(`delete from frontdesk.health_checks where checked_at < now() - make_interval(days => $1)`, days)
	if err != nil {
		return err
	}
	_, err = conn.Exec(`delete from frontdesk.host_metrics where sampled_at < now() - make_interval(days => $1)`, days)
	return err
}
