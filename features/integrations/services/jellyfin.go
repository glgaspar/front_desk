package services

import (
	"fmt"
	"time"
)

type Stream struct {
	User     string  `json:"user"`
	Device   string  `json:"device"`
	Client   string  `json:"client"`
	Title    string  `json:"title"`
	Series   string  `json:"series"`
	Type     string  `json:"type"`
	Paused   bool    `json:"paused"`
	Progress float64 `json:"progress"`
}

type jellyfinItem struct {
	Name              string    `json:"Name"`
	SeriesName        string    `json:"SeriesName"`
	Type              string    `json:"Type"`
	RunTimeTicks      int64     `json:"RunTimeTicks"`
	ParentIndexNumber int       `json:"ParentIndexNumber"`
	IndexNumber       int       `json:"IndexNumber"`
	DateCreated       time.Time `json:"DateCreated"`
}

func jellyfin(c *Config, path string, out any) error {
	return call(client, "GET", c.Url+path, map[string]string{"X-Emby-Token": c.Secret}, nil, out)
}

func checkJellyfin(c Config) error {
	return jellyfin(&c, "/System/Info", nil)
}

// only the sessions that are actually playing something
func JellyfinStreams() ([]Stream, error) {
	c, err := loadEnabled("jellyfin")
	if err != nil {
		return nil, err
	}

	var sessions []struct {
		UserName       string        `json:"UserName"`
		DeviceName     string        `json:"DeviceName"`
		Client         string        `json:"Client"`
		NowPlayingItem *jellyfinItem `json:"NowPlayingItem"`
		PlayState      struct {
			PositionTicks int64 `json:"PositionTicks"`
			IsPaused      bool  `json:"IsPaused"`
		} `json:"PlayState"`
	}
	err = jellyfin(c, "/Sessions?activeWithinSeconds=600", &sessions)
	if err != nil {
		return nil, fmt.Errorf("jellyfin: %w", err)
	}

	streams := []Stream{}
	for _, s := range sessions {
		if s.NowPlayingItem == nil {
			continue
		}
		stream := Stream{
			User:   s.UserName,
			Device: s.DeviceName,
			Client: s.Client,
			Title:  episodeTitle(*s.NowPlayingItem),
			Series: s.NowPlayingItem.SeriesName,
			Type:   s.NowPlayingItem.Type,
			Paused: s.PlayState.IsPaused,
		}
		if s.NowPlayingItem.RunTimeTicks > 0 {
			stream.Progress = float64(s.PlayState.PositionTicks) / float64(s.NowPlayingItem.RunTimeTicks)
		}
		streams = append(streams, stream)
	}
	return streams, nil
}

func episodeTitle(item jellyfinItem) string {
	if item.Type == "Episode" {
		return fmt.Sprintf("S%02dE%02d %s", item.ParentIndexNumber, item.IndexNumber, item.Name)
	}
	return item.Name
}

func jellyfinRecent(c *Config) ([]PipelineItem, error) {
	var res struct {
		Items []jellyfinItem `json:"Items"`
	}
	err := jellyfin(c, "/Items?SortBy=DateCreated&SortOrder=Descending&Recursive=true&IncludeItemTypes=Movie,Episode&Limit=8&Fields=DateCreated", &res)
	if err != nil {
		return nil, fmt.Errorf("jellyfin: %w", err)
	}

	items := []PipelineItem{}
	for _, item := range res.Items {
		title := item.Name
		if item.Type == "Episode" {
			title = item.SeriesName + " " + episodeTitle(item)
		}
		items = append(items, PipelineItem{Title: title, Source: "jellyfin", Date: item.DateCreated.Format(time.RFC3339)})
	}
	return items, nil
}
