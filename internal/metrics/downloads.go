package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cagridursun/devcade/internal/profile"
)

type DownloadSample struct {
	At        time.Time        `json:"at"`
	Total     int64            `json:"total"`
	Platforms map[string]int64 `json:"platforms"`
	Versions  map[string]int64 `json:"versions"`
}
type DownloadReport struct {
	Samples []DownloadSample `json:"samples"`
	Error   string           `json:"error,omitempty"`
}
type DownloadStore struct {
	mu     sync.Mutex
	path   string
	report DownloadReport
}

func OpenDownloads(path string) (*DownloadStore, error) {
	d := &DownloadStore{path: path, report: DownloadReport{Samples: []DownloadSample{}}}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) > 4<<20 {
		return nil, fmt.Errorf("download history too large")
	}
	if err = json.Unmarshal(b, &d.report); err != nil {
		return nil, err
	}
	if len(d.report.Samples) > 2160 {
		return nil, fmt.Errorf("invalid download history")
	}
	return d, nil
}
func (d *DownloadStore) Report() DownloadReport {
	d.mu.Lock()
	defer d.mu.Unlock()
	r := d.report
	r.Samples = append([]DownloadSample{}, r.Samples...)
	return r
}
func (d *DownloadStore) Poll(ctx context.Context, client *http.Client, base string) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	sample := DownloadSample{At: time.Now().UTC(), Platforms: map[string]int64{}, Versions: map[string]int64{}}
	// Follow up to ten pages of published releases. Refuse partial totals.
	for page := 1; page <= 10; page++ {
		req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/repos/cagridursun/devcade/releases?per_page=100&page=%d", strings.TrimRight(base, "/"), page), nil)
		if err != nil {
			return err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "DevCade-download-statistics")
		response, err := client.Do(req)
		if err != nil {
			return d.failed(err)
		}
		if response.StatusCode != 200 {
			response.Body.Close()
			return d.failed(fmt.Errorf("GitHub HTTP %d", response.StatusCode))
		}
		var releases []struct {
			Tag    string `json:"tag_name"`
			Draft  bool   `json:"draft"`
			Assets []struct {
				Name  string `json:"name"`
				Count int64  `json:"download_count"`
			} `json:"assets"`
		}
		err = json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&releases)
		response.Body.Close()
		if err != nil {
			return d.failed(err)
		}
		for _, release := range releases {
			if release.Draft {
				continue
			}
			for _, asset := range release.Assets {
				platform := ""
				if strings.HasPrefix(asset.Name, "devcade_") && (strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".zip")) {
					for _, p := range []string{"darwin", "linux", "windows"} {
						if strings.Contains(asset.Name, "_"+p+"_") {
							platform = p
							break
						}
					}
				}
				if platform == "" {
					continue
				}
				if asset.Count < 0 {
					return d.failed(fmt.Errorf("invalid download count"))
				}
				sample.Total += asset.Count
				sample.Platforms[platform] += asset.Count
				sample.Versions[release.Tag] += asset.Count
			}
		}
		if len(releases) < 100 {
			break
		}
		if page == 10 {
			return d.failed(fmt.Errorf("too many releases; refusing partial download count"))
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	next := DownloadReport{Samples: append(append([]DownloadSample{}, d.report.Samples...), sample)}
	if len(next.Samples) > 2160 {
		next.Samples = next.Samples[len(next.Samples)-2160:]
	}
	b, err := json.Marshal(next)
	if err == nil {
		err = profile.Write(d.path, b)
	}
	if err != nil {
		return err
	}
	d.report = next
	return nil
}
func (d *DownloadStore) failed(err error) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.report.Error = "Last GitHub refresh failed; previous sample retained."
	return err
}
