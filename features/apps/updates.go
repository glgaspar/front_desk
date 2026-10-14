package apps

import (
	"context"
	"errors"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/glgaspar/front_desk/connection"
)

type UpdateStatus struct {
	ImageRef     string    `json:"imageRef"`
	LocalDigest  string    `json:"localDigest"`
	RemoteDigest string    `json:"remoteDigest"`
	Status       string    `json:"status"` // up_to_date | update_available | unsupported | rate_limited | error
	Error        string    `json:"error"`
	CheckedAt    time.Time `json:"checkedAt"`
}

func updateTTL() time.Duration {
	ttl, err := time.ParseDuration(os.Getenv("UPDATE_CHECK_TTL"))
	if err != nil || ttl <= 0 {
		return 6 * time.Hour
	}
	return ttl
}

// asking the registry every time would get us rate limited real fast, so it lives in the db for a while
func CheckUpdate(ref string, force bool) (UpdateStatus, error) {
	status := UpdateStatus{ImageRef: ref}
	if ref == "" {
		return status, fmt.Errorf("app has no image")
	}

	conn, err := connection.Db()
	if err != nil {
		return status, err
	}

	if !force {
		query := `
		select coalesce(local_digest, ''), coalesce(remote_digest, ''), status, coalesce(error, ''), checked_at
		from frontdesk.image_update_checks
		where image_ref = $1 and checked_at > $2`
		err = conn.QueryRow(query, ref, time.Now().Add(-updateTTL())).Scan(&status.LocalDigest, &status.RemoteDigest, &status.Status, &status.Error, &status.CheckedAt)
		if err == nil {
			return status, nil
		}
		if err != sql.ErrNoRows {
			return status, err
		}
	}

	status = askRegistry(ref)

	query := `
	insert into frontdesk.image_update_checks (image_ref, local_digest, remote_digest, status, error, checked_at)
	values ($1, $2, $3, $4, $5, $6)
	on conflict (image_ref) do update set
		local_digest = excluded.local_digest,
		remote_digest = excluded.remote_digest,
		status = excluded.status,
		error = excluded.error,
		checked_at = excluded.checked_at`
	_, err = conn.Exec(query, status.ImageRef, status.LocalDigest, status.RemoteDigest, status.Status, status.Error, status.CheckedAt)
	if err != nil {
		return status, err
	}

	return status, nil
}

func askRegistry(ref string) UpdateStatus {
	status := UpdateStatus{ImageRef: ref, CheckedAt: time.Now()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	output, err := runDocker(ctx, "", "image", "inspect", "--format", "{{json .RepoDigests}}", ref)
	if err != nil {
		status.Status = "error"
		status.Error = strings.TrimSpace(err.Error())
		return status
	}

	var localDigests []string
	json.Unmarshal(output, &localDigests)
	if len(localDigests) == 0 {
		// built here, there is no registry to ask
		status.Status = "unsupported"
		return status
	}
	_, status.LocalDigest, _ = strings.Cut(localDigests[0], "@")

	remote, err := remoteDigest(ctx, ref)
	if errors.Is(err, errPinned) {
		status.Status = "unsupported"
		return status
	}
	if errors.Is(err, errRateLimited) {
		status.Status = "rate_limited"
		status.Error = err.Error()
		return status
	}
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status
	}
	status.RemoteDigest = remote

	status.Status = "update_available"
	for _, local := range localDigests {
		if strings.HasSuffix(local, "@"+remote) {
			status.Status = "up_to_date"
			status.LocalDigest = remote
			break
		}
	}

	return status
}

func ForgetUpdate(ref string) {
	conn, err := connection.Db()
	if err != nil {
		return
	}
	conn.Exec(`delete from frontdesk.image_update_checks where image_ref = $1`, ref)
}

func GetAllUpdates(force bool) (map[string]UpdateStatus, error) {
	appList, err := new(App).GetList()
	if err != nil {
		return nil, err
	}

	refs := map[string]bool{}
	for _, app := range appList {
		if app.ImageRef != "" {
			refs[app.ImageRef] = true
		}
	}

	result := map[string]UpdateStatus{}
	var lock sync.Mutex
	var wg sync.WaitGroup
	limit := make(chan struct{}, 2)

	for ref := range refs {
		wg.Add(1)
		go func(ref string) {
			defer wg.Done()
			limit <- struct{}{}
			defer func() { <-limit }()

			status, err := CheckUpdate(ref, force)
			if err != nil {
				status.Status = "error"
				status.Error = err.Error()
			}
			lock.Lock()
			result[ref] = status
			lock.Unlock()
		}(ref)
	}
	wg.Wait()

	return result, nil
}
