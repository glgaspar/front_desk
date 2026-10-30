package widgets

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/glgaspar/front_desk/connection"
)

// a widget that talks to some service declares what it needs in its own file (the manifest).
// an admin fills the fields on the widgets page and from then on the widget asks the api to
// make the calls. the api only knows what the manifest says, nothing about the service itself

type Field struct {
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Secret      bool   `json:"secret"`
	Optional    bool   `json:"optional"`
}

type Rule struct {
	Method string `json:"method"`
	Path   string `json:"path"`  // prefix
	Admin  bool   `json:"admin"` // for the calls that change something on the other side
}

type Manifest struct {
	Fields map[string]Field `json:"fields"`
	Auth   struct {
		Headers map[string]string `json:"headers"`
		Query   map[string]string `json:"query"`
		Basic   *struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"basic"`
	} `json:"auth"`
	Allow []Rule `json:"allow"`
}

var validName = regexp.MustCompile(`^[a-z0-9_]{1,50}$`)

func ValidName(name string) bool {
	return validName.MatchString(name)
}

func parseManifest(text string) (*Manifest, error) {
	m := new(Manifest)
	if err := json.Unmarshal([]byte(text), m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	if _, ok := m.Fields["url"]; !ok {
		return nil, fmt.Errorf("the manifest needs a url field, its where the calls go")
	}
	// nothing declared means it can only read
	if len(m.Allow) == 0 {
		m.Allow = []Rule{{Method: "GET", Path: "/"}}
	}
	return m, nil
}

func baseUrl(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimRight(strings.TrimSpace(raw), "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("url must look like http://192.168.0.10:8265")
	}
	return u, nil
}

// the manifest is saved together with the values and only an admin can save, so whatever the
// manifest says is what an admin agreed to. the front never sends it again on a call
func SaveConfig(name string, manifestText string, values map[string]string) error {
	m, err := parseManifest(manifestText)
	if err != nil {
		return err
	}

	old := map[string]string{}
	if _, oldValues, err := loadConfig(name); err == nil {
		old = oldValues
	}

	clean := map[string]string{}
	for key, field := range m.Fields {
		value := strings.TrimSpace(values[key])
		// same as the other configs, an empty secret keeps the saved one
		if value == "" && field.Secret {
			value = old[key]
		}
		if value == "" && !field.Optional {
			return fmt.Errorf("%s is required", field.Label)
		}
		clean[key] = value
	}
	base, err := baseUrl(clean["url"])
	if err != nil {
		return err
	}
	clean["url"] = base.String()

	conn, err := connection.Db()
	if err != nil {
		return err
	}
	valuesJson, _ := json.Marshal(clean)
	query := `
	insert into frontdesk.widget_configs (name, manifest, "values")
	values ($1, $2, $3)
	on conflict (name) do update set manifest = excluded.manifest, "values" = excluded."values", updated_at = now()`
	_, err = conn.Exec(query, name, manifestText, valuesJson)
	return err
}

func loadConfig(name string) (string, map[string]string, error) {
	conn, err := connection.Db()
	if err != nil {
		return "", nil, err
	}
	var manifest string
	var raw []byte
	err = conn.QueryRow(`select manifest, "values" from frontdesk.widget_configs where name = $1`, name).Scan(&manifest, &raw)
	if err == sql.ErrNoRows {
		return "", nil, fmt.Errorf("not configured yet")
	}
	if err != nil {
		return "", nil, err
	}
	values := map[string]string{}
	err = json.Unmarshal(raw, &values)
	return manifest, values, err
}

type PublicWidgetConfig struct {
	Manifest   string            `json:"manifest"`
	Values     map[string]string `json:"values"`
	SecretsSet []string          `json:"secretsSet"`
}

// secrets never leave the api, the front only learns if they are set
func PublicConfig(name string) (*PublicWidgetConfig, error) {
	manifestText, values, err := loadConfig(name)
	if err != nil {
		return nil, err
	}
	m, err := parseManifest(manifestText)
	if err != nil {
		return nil, err
	}

	public := &PublicWidgetConfig{Manifest: manifestText, Values: map[string]string{}, SecretsSet: []string{}}
	for key, value := range values {
		if m.Fields[key].Secret {
			if value != "" {
				public.SecretsSet = append(public.SecretsSet, key)
			}
			continue
		}
		public.Values[key] = value
	}
	return public, nil
}

var placeholder = regexp.MustCompile(`\{(\w+)\}`)

func fill(template string, values map[string]string) string {
	return placeholder.ReplaceAllStringFunc(template, func(match string) string {
		return values[match[1:len(match)-1]]
	})
}

func (m *Manifest) rule(method string, path string) *Rule {
	clean := strings.SplitN(path, "?", 2)[0]
	for i, r := range m.Allow {
		if strings.EqualFold(r.Method, method) && strings.HasPrefix(clean, r.Path) {
			return &m.Allow[i]
		}
	}
	return nil
}

// everything that decides where the call goes and what goes with it. no db and no network here
func buildRequest(m *Manifest, values map[string]string, method string, path string, body []byte, isAdmin bool) (*http.Request, *Rule, error) {
	method = strings.ToUpper(method)
	if !strings.HasPrefix(path, "/") || strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return nil, nil, fmt.Errorf("path must start with / and cant go up a folder")
	}

	rule := m.rule(method, path)
	if rule == nil {
		return nil, nil, fmt.Errorf("this widget is not allowed to %s %s", method, path)
	}
	if rule.Admin && !isAdmin {
		return nil, nil, fmt.Errorf("only admins can do that")
	}

	base, err := baseUrl(values["url"])
	if err != nil {
		return nil, nil, err
	}
	// glued to the saved url and checked again, whatever the path has it cant leave that host
	target, err := url.Parse(base.String() + path)
	if err != nil || target.Scheme != base.Scheme || target.Host != base.Host {
		return nil, nil, fmt.Errorf("invalid path")
	}

	query := target.Query()
	for key, template := range m.Auth.Query {
		if value := fill(template, values); value != "" {
			query.Set(key, value)
		}
	}
	target.RawQuery = query.Encode()

	var reader io.Reader
	if len(body) > 0 && string(body) != "null" {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, target.String(), reader)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if reader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, template := range m.Auth.Headers {
		// an optional key that is not set just doesnt go, instead of going empty
		if value := fill(template, values); strings.TrimSpace(strings.TrimPrefix(value, "Bearer")) != "" {
			req.Header.Set(key, value)
		}
	}
	if m.Auth.Basic != nil {
		req.SetBasicAuth(fill(m.Auth.Basic.Username, values), fill(m.Auth.Basic.Password, values))
	}
	return req, rule, nil
}

var client = &http.Client{
	Timeout: 15 * time.Second,
	// a redirect could take it to another host, the widget gets the redirect instead
	CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse },
}

const maxResponse = 2 << 20

type CallResult struct {
	Status int
	Body   []byte
	Logged bool // only the admin calls change something, the rest is polling
}

func Call(ctx context.Context, name string, method string, path string, body []byte, isAdmin bool) (*CallResult, error) {
	manifestText, values, err := loadConfig(name)
	if err != nil {
		return nil, err
	}
	m, err := parseManifest(manifestText)
	if err != nil {
		return nil, err
	}

	req, rule, err := buildRequest(m, values, method, path, body, isAdmin)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(io.LimitReader(res.Body, maxResponse+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponse {
		return nil, fmt.Errorf("the answer is bigger than 2MB, ask for less")
	}
	return &CallResult{Status: res.StatusCode, Body: data, Logged: rule.Admin}, nil
}
