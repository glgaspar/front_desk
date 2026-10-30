package services

import "fmt"

type TdarrStats struct {
	Queue     int     `json:"queue"`
	Processed int     `json:"processed"`
	Errored   int     `json:"errored"`
	SavedGB   float64 `json:"savedGB"`
	Files     int     `json:"files"`
}

// tdarr keeps the totals in a "statistics" doc. tables 1-3 are transcodes, 4-6 health checks:
// queue, done, error. same math homepage uses
func tdarrStats(c *Config) (*TdarrStats, error) {
	var raw struct {
		TotalFileCount int     `json:"totalFileCount"`
		SizeDiff       float64 `json:"sizeDiff"`
		Table1Count    int     `json:"table1Count"`
		Table2Count    int     `json:"table2Count"`
		Table3Count    int     `json:"table3Count"`
		Table4Count    int     `json:"table4Count"`
		Table5Count    int     `json:"table5Count"`
		Table6Count    int     `json:"table6Count"`
	}
	headers := map[string]string{}
	if c.Secret != "" {
		headers["x-api-key"] = c.Secret
	}
	body := map[string]interface{}{
		"data": map[string]string{"collection": "StatisticsJSONDB", "mode": "getById", "docID": "statistics"},
	}
	err := call(client, "POST", c.Url+"/api/v2/cruddb", headers, body, &raw)
	if err != nil {
		return nil, fmt.Errorf("tdarr: %w", err)
	}

	return &TdarrStats{
		Queue:     raw.Table1Count + raw.Table4Count,
		Processed: raw.Table2Count + raw.Table5Count,
		Errored:   raw.Table3Count + raw.Table6Count,
		SavedGB:   raw.SizeDiff,
		Files:     raw.TotalFileCount,
	}, nil
}

func checkTdarr(c Config) error {
	_, err := tdarrStats(&c)
	return err
}

func GetTdarrStats() (*TdarrStats, error) {
	c, err := loadEnabled("tdarr")
	if err != nil {
		return nil, err
	}
	return tdarrStats(c)
}
