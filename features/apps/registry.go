package apps

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// docker hub counts every manifest GET as a pull, and anonymous pulls are like 10 per hour.
// HEAD does not count, so this only ever does HEAD on the manifests
var errRateLimited = errors.New("registry is rate limiting this server, will try again later")
var errPinned = errors.New("image is pinned to a digest, nothing to update")

var registryClient = &http.Client{Timeout: 15 * time.Second}

// so the tests can point it to a local server
var registryBase = func(host string) string {
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		return "http://" + host
	}
	return "https://" + host
}

const manifestAccept = "application/vnd.oci.image.index.v1+json, " +
	"application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.oci.image.manifest.v1+json, " +
	"application/vnd.docker.distribution.manifest.v2+json"

// nginx -> registry-1.docker.io, library/nginx, latest
func parseRef(ref string) (string, string, string, error) {
	if strings.Contains(ref, "@") {
		return "", "", "", errPinned
	}

	registry := "registry-1.docker.io"
	rest := ref
	if first, after, found := strings.Cut(ref, "/"); found && (strings.ContainsAny(first, ".:") || first == "localhost") {
		registry = first
		rest = after
	}
	if registry == "docker.io" || registry == "index.docker.io" {
		registry = "registry-1.docker.io"
	}

	repo := rest
	tag := "latest"
	// the tag is after the last colon, as long as that colon is not part of a path
	if i := strings.LastIndex(rest, ":"); i > strings.LastIndex(rest, "/") {
		repo = rest[:i]
		tag = rest[i+1:]
	}

	if registry == "registry-1.docker.io" && !strings.Contains(repo, "/") {
		repo = "library/" + repo
	}
	if repo == "" || tag == "" {
		return "", "", "", fmt.Errorf("could not understand image %s", ref)
	}

	return registry, repo, tag, nil
}

// Bearer realm="https://auth.docker.io/token",service="registry.docker.io",scope="repository:library/nginx:pull"
func parseAuthenticate(header string) map[string]string {
	values := map[string]string{}
	scheme, params, found := strings.Cut(strings.TrimSpace(header), " ")
	if !found || !strings.EqualFold(scheme, "bearer") {
		return values
	}

	for params != "" {
		params = strings.TrimLeft(params, ", ")
		key, rest, ok := strings.Cut(params, "=")
		if !ok {
			break
		}
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				break
			}
			value = rest[1 : end+1]
			params = rest[end+2:]
		} else {
			value, params, _ = strings.Cut(rest, ",")
		}
		values[strings.ToLower(strings.TrimSpace(key))] = value
	}
	return values
}

func anonymousToken(ctx context.Context, header string) (string, error) {
	auth := parseAuthenticate(header)
	if auth["realm"] == "" {
		return "", fmt.Errorf("registry asked for a login we dont know how to do")
	}

	u, err := url.Parse(auth["realm"])
	if err != nil {
		return "", err
	}
	q := u.Query()
	if auth["service"] != "" {
		q.Set("service", auth["service"])
	}
	if auth["scope"] != "" {
		q.Set("scope", auth["scope"])
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return "", err
	}
	res, err := registryClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusTooManyRequests {
		return "", errRateLimited
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("registry token request failed with status %d", res.StatusCode)
	}

	var parsed struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(res.Body).Decode(&parsed)
	if err != nil {
		return "", err
	}
	if parsed.Token != "" {
		return parsed.Token, nil
	}
	return parsed.AccessToken, nil
}

func headManifest(ctx context.Context, manifestUrl string, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", manifestUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", manifestAccept)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := registryClient.Do(req)
	if err != nil {
		return nil, err
	}
	res.Body.Close()
	return res, nil
}

func remoteDigest(ctx context.Context, ref string) (string, error) {
	registry, repo, tag, err := parseRef(ref)
	if err != nil {
		return "", err
	}

	manifestUrl := fmt.Sprintf("%s/v2/%s/manifests/%s", registryBase(registry), repo, tag)
	res, err := headManifest(ctx, manifestUrl, "")
	if err != nil {
		return "", err
	}

	// almost every registry wants a token even for public images, anonymous ones are free
	if res.StatusCode == http.StatusUnauthorized {
		token, err := anonymousToken(ctx, res.Header.Get("Www-Authenticate"))
		if err != nil {
			return "", err
		}
		res, err = headManifest(ctx, manifestUrl, token)
		if err != nil {
			return "", err
		}
	}

	switch {
	case res.StatusCode == http.StatusTooManyRequests:
		return "", errRateLimited
	case res.StatusCode == http.StatusNotFound:
		return "", fmt.Errorf("%s:%s was not found on %s", repo, tag, registry)
	case res.StatusCode != http.StatusOK:
		return "", fmt.Errorf("registry answered with status %d", res.StatusCode)
	}

	digest := res.Header.Get("Docker-Content-Digest")
	if digest == "" {
		return "", fmt.Errorf("registry did not send the digest")
	}
	return digest, nil
}
