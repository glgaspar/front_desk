package services

import (
	"fmt"
	"net/url"
	"sort"
	"sync"
	"time"
)

type CalendarItem struct {
	Title   string    `json:"title"`
	Date    time.Time `json:"date"`
	Source  string    `json:"source"`
	HasFile bool      `json:"hasFile"`
}

type Calendar struct {
	Items  []CalendarItem `json:"items"`
	Errors []string       `json:"errors"`
}

func calendarRange(days int) string {
	start := time.Now().Truncate(24 * time.Hour)
	q := url.Values{
		"start": {start.Format(time.RFC3339)},
		"end":   {start.AddDate(0, 0, days).Format(time.RFC3339)},
	}
	return q.Encode()
}

func sonarrCalendar(c *Config, days int) ([]CalendarItem, error) {
	var episodes []struct {
		Title         string    `json:"title"`
		SeasonNumber  int       `json:"seasonNumber"`
		EpisodeNumber int       `json:"episodeNumber"`
		AirDateUtc    time.Time `json:"airDateUtc"`
		HasFile       bool      `json:"hasFile"`
		Series        struct {
			Title string `json:"title"`
		} `json:"series"`
	}
	err := arr(c, "/api/v3/calendar?includeSeries=true&"+calendarRange(days), &episodes)
	if err != nil {
		return nil, fmt.Errorf("sonarr: %w", err)
	}

	items := []CalendarItem{}
	for _, e := range episodes {
		items = append(items, CalendarItem{
			Title:   fmt.Sprintf("%s S%02dE%02d", e.Series.Title, e.SeasonNumber, e.EpisodeNumber),
			Date:    e.AirDateUtc,
			Source:  "sonarr",
			HasFile: e.HasFile,
		})
	}
	return items, nil
}

func radarrCalendar(c *Config, days int) ([]CalendarItem, error) {
	var movies []struct {
		Title           string     `json:"title"`
		Year            int        `json:"year"`
		InCinemas       *time.Time `json:"inCinemas"`
		DigitalRelease  *time.Time `json:"digitalRelease"`
		PhysicalRelease *time.Time `json:"physicalRelease"`
		HasFile         bool       `json:"hasFile"`
	}
	err := arr(c, "/api/v3/calendar?"+calendarRange(days), &movies)
	if err != nil {
		return nil, fmt.Errorf("radarr: %w", err)
	}

	// radarr returns the movie if any of the dates is in the range, the one in range is the one that matters
	start := time.Now().Truncate(24 * time.Hour)
	end := start.AddDate(0, 0, days)
	items := []CalendarItem{}
	for _, m := range movies {
		for _, release := range []struct {
			date *time.Time
			kind string
		}{{m.DigitalRelease, "digital"}, {m.PhysicalRelease, "physical"}, {m.InCinemas, "cinemas"}} {
			if release.date != nil && !release.date.Before(start) && release.date.Before(end) {
				items = append(items, CalendarItem{
					Title:   fmt.Sprintf("%s (%d) %s", m.Title, m.Year, release.kind),
					Date:    *release.date,
					Source:  "radarr",
					HasFile: m.HasFile,
				})
				break
			}
		}
	}
	return items, nil
}

func MediaCalendar(days int) Calendar {
	cal := Calendar{Items: []CalendarItem{}, Errors: []string{}}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, fetch := range map[string]func(*Config, int) ([]CalendarItem, error){"sonarr": sonarrCalendar, "radarr": radarrCalendar} {
		c, err := loadEnabled(name)
		if err != nil {
			continue
		}
		wg.Add(1)
		go func(fetch func(*Config, int) ([]CalendarItem, error)) {
			defer wg.Done()
			items, err := fetch(c, days)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				cal.Errors = append(cal.Errors, err.Error())
				return
			}
			cal.Items = append(cal.Items, items...)
		}(fetch)
	}
	wg.Wait()

	sort.Slice(cal.Items, func(i, j int) bool { return cal.Items[i].Date.Before(cal.Items[j].Date) })
	return cal
}
