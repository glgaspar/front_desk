package services

import (
	"fmt"
	"sync"
)

// one item anywhere in the way from "someone asked for it" to "its on jellyfin"
type PipelineItem struct {
	Title    string  `json:"title"`
	Source   string  `json:"source"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	TimeLeft string  `json:"timeLeft"`
	Date     string  `json:"date"`
}

type Pipeline struct {
	Requests  []PipelineItem `json:"requests"`
	Downloads []PipelineItem `json:"downloads"`
	Recent    []PipelineItem `json:"recent"`
	Errors    []string       `json:"errors"`
}

// asks everything that is configured at the same time. one of them being down
// only shows up in errors, the rest still comes back
func GetPipeline() Pipeline {
	p := Pipeline{Requests: []PipelineItem{}, Downloads: []PipelineItem{}, Recent: []PipelineItem{}, Errors: []string{}}
	var mu sync.Mutex
	var wg sync.WaitGroup

	run := func(name string, fetch func(*Config) ([]PipelineItem, error), add func([]PipelineItem)) {
		c, err := loadEnabled(name)
		if err != nil {
			return
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			items, err := fetch(c)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				p.Errors = append(p.Errors, err.Error())
				return
			}
			add(items)
		}()
	}

	run("seerr", seerrRequests, func(items []PipelineItem) { p.Requests = append(p.Requests, items...) })
	run("sonarr", sonarrQueue, func(items []PipelineItem) { p.Downloads = append(p.Downloads, items...) })
	run("radarr", radarrQueue, func(items []PipelineItem) { p.Downloads = append(p.Downloads, items...) })
	run("jellyfin", jellyfinRecent, func(items []PipelineItem) { p.Recent = append(p.Recent, items...) })
	wg.Wait()

	return p
}

func arr(c *Config, path string, out any) error {
	return call(client, "GET", c.Url+path, map[string]string{"X-Api-Key": c.Secret}, nil, out)
}

func checkArr(c Config) error {
	return arr(&c, "/api/v3/system/status", nil)
}

type arrQueue struct {
	Records []struct {
		Title    string  `json:"title"`
		Size     float64 `json:"size"`
		SizeLeft float64 `json:"sizeleft"`
		TimeLeft string  `json:"timeleft"`
		Status   string  `json:"status"`
		Series   *struct {
			Title string `json:"title"`
		} `json:"series"`
		Episode *struct {
			SeasonNumber  int `json:"seasonNumber"`
			EpisodeNumber int `json:"episodeNumber"`
		} `json:"episode"`
		Movie *struct {
			Title string `json:"title"`
			Year  int    `json:"year"`
		} `json:"movie"`
	} `json:"records"`
}

func queue(c *Config, source string, path string) ([]PipelineItem, error) {
	var q arrQueue
	err := arr(c, path, &q)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", source, err)
	}

	items := []PipelineItem{}
	for _, r := range q.Records {
		item := PipelineItem{Title: r.Title, Source: source, Status: r.Status, TimeLeft: r.TimeLeft}
		if r.Series != nil {
			item.Title = r.Series.Title
			if r.Episode != nil {
				item.Title += fmt.Sprintf(" S%02dE%02d", r.Episode.SeasonNumber, r.Episode.EpisodeNumber)
			}
		}
		if r.Movie != nil {
			item.Title = fmt.Sprintf("%s (%d)", r.Movie.Title, r.Movie.Year)
		}
		if r.Size > 0 {
			item.Progress = 1 - r.SizeLeft/r.Size
		}
		items = append(items, item)
	}
	return items, nil
}

func sonarrQueue(c *Config) ([]PipelineItem, error) {
	return queue(c, "sonarr", "/api/v3/queue?pageSize=50&includeSeries=true&includeEpisode=true")
}

func radarrQueue(c *Config) ([]PipelineItem, error) {
	return queue(c, "radarr", "/api/v3/queue?pageSize=50&includeMovie=true")
}

func seerr(c *Config, path string, out any) error {
	return call(client, "GET", c.Url+"/api/v1"+path, map[string]string{"X-Api-Key": c.Secret}, nil, out)
}

func checkSeerr(c Config) error {
	return seerr(&c, "/request?take=1", nil)
}

type seerrRequest struct {
	Id        int    `json:"id"`
	Status    int    `json:"status"`
	Type      string `json:"type"`
	CreatedAt string `json:"createdAt"`
	Media     struct {
		TmdbId int `json:"tmdbId"`
	} `json:"media"`
	RequestedBy struct {
		DisplayName string `json:"displayName"`
	} `json:"requestedBy"`
}

// the request list only has the tmdb id, so the titles cost one call each. thats why only a few at a time
func seerrTitles(c *Config, requests []seerrRequest) []string {
	titles := make([]string, len(requests))
	var wg sync.WaitGroup
	for i, r := range requests {
		titles[i] = fmt.Sprintf("tmdb %d", r.Media.TmdbId)
		wg.Add(1)
		go func(i int, r seerrRequest) {
			defer wg.Done()
			var media struct {
				Title string `json:"title"`
				Name  string `json:"name"`
			}
			if seerr(c, fmt.Sprintf("/%s/%d", r.Type, r.Media.TmdbId), &media) != nil {
				return
			}
			if media.Title != "" {
				titles[i] = media.Title
			} else if media.Name != "" {
				titles[i] = media.Name
			}
		}(i, r)
	}
	wg.Wait()
	return titles
}

func seerrList(c *Config, filter string) ([]seerrRequest, error) {
	var res struct {
		Results []seerrRequest `json:"results"`
	}
	err := seerr(c, "/request?take=8&sort=added&filter="+filter, &res)
	if err != nil {
		return nil, fmt.Errorf("seerr: %w", err)
	}
	return res.Results, nil
}

func seerrRequests(c *Config) ([]PipelineItem, error) {
	requests, err := seerrList(c, "unavailable")
	if err != nil {
		return nil, err
	}

	titles := seerrTitles(c, requests)
	items := []PipelineItem{}
	for i, r := range requests {
		status := "approved"
		if r.Status == 1 {
			status = "waiting approval"
		}
		items = append(items, PipelineItem{Title: titles[i], Source: "seerr", Status: status})
	}
	return items, nil
}

type SeerrPending struct {
	Id          int    `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	RequestedBy string `json:"requestedBy"`
	CreatedAt   string `json:"createdAt"`
}

func SeerrPendingRequests() ([]SeerrPending, error) {
	c, err := loadEnabled("seerr")
	if err != nil {
		return nil, err
	}
	requests, err := seerrList(c, "pending")
	if err != nil {
		return nil, err
	}

	titles := seerrTitles(c, requests)
	pending := []SeerrPending{}
	for i, r := range requests {
		pending = append(pending, SeerrPending{Id: r.Id, Title: titles[i], Type: r.Type, RequestedBy: r.RequestedBy.DisplayName, CreatedAt: r.CreatedAt})
	}
	return pending, nil
}

// action is approve or decline, same words seerr uses on the url
func SeerrAnswer(id int, action string) error {
	c, err := loadEnabled("seerr")
	if err != nil {
		return err
	}
	err = call(client, "POST", fmt.Sprintf("%s/api/v1/request/%d/%s", c.Url, id, action), map[string]string{"X-Api-Key": c.Secret}, nil, nil)
	if err != nil {
		return fmt.Errorf("seerr: %w", err)
	}
	return nil
}
