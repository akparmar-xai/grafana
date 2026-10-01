package dtos

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/grafana/grafana/pkg/apimachinery/identity"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/setting"
)

var regNonAlphaNumeric = regexp.MustCompile("[^a-zA-Z0-9]+")

type AnyId struct {
	Id int64 `json:"id"`
}

type LoginCommand struct {
	User     string `json:"user" binding:"Required"`
	Password string `json:"password" binding:"Required"`
	Remember bool   `json:"remember"`
}

type CurrentUser struct {
	IsSignedIn                 bool               `json:"isSignedIn"`
	Id                         int64              `json:"id"`
	UID                        string             `json:"uid"`
	Login                      string             `json:"login"`
	Email                      string             `json:"email"`
	Name                       string             `json:"name"`
	Theme                      string             `json:"theme"`
	LightTheme                 bool               `json:"lightTheme"` // deprecated, use theme instead
	OrgCount                   int                `json:"orgCount"`
	OrgId                      int64              `json:"orgId"`
	OrgName                    string             `json:"orgName"`
	OrgRole                    org.RoleType       `json:"orgRole"`
	IsGrafanaAdmin             bool               `json:"isGrafanaAdmin"`
	GravatarUrl                string             `json:"gravatarUrl"`
	Timezone                   string             `json:"timezone"`
	WeekStart                  string             `json:"weekStart"`
	Locale                     string             `json:"locale"`
	Language                   string             `json:"language"`
	HasEditPermissionInFolders bool               `json:"hasEditPermissionInFolders"`
	AuthenticatedBy            string             `json:"authenticatedBy"`
	Permissions                UserPermissionsMap `json:"permissions,omitempty"`
	Analytics                  AnalyticsSettings  `json:"analytics"`
}

type AnalyticsSettings struct {
	Identifier string `json:"identifier"`
}

type UserPermissionsMap map[string]bool

// swagger:model
type MetricRequest struct {
	// From Start time in epoch timestamps in milliseconds or relative using Grafana time units.
	// required: true
	// example: now-1h
	From string `json:"from"`
	// To End time in epoch timestamps in milliseconds or relative using Grafana time units.
	// required: true
	// example: now
	To string `json:"to"`
	// queries.refId – Specifies an identifier of the query. Is optional and default to “A”.
	// queries.datasourceId – Specifies the data source to be queried. Each query in the request must have an unique datasourceId.
	// queries.maxDataPoints - Species maximum amount of data points that dashboard panel can render. Is optional and default to 100.
	// queries.intervalMs - Specifies the time interval in milliseconds of time series. Is optional and defaults to 1000.
	// required: true
	// example: [ { "refId": "A", "intervalMs": 86400000, "maxDataPoints": 1092, "datasource":{ "uid":"PD8C576611E62080A" }, "rawSql": "SELECT 1 as valueOne, 2 as valueTwo", "format": "table" } ]
	Queries []*simplejson.Json `json:"queries"`
	// required: false
	Debug bool `json:"debug"`

	// rawQueries holds the original JSON of each query, parallel to Queries, when the
	// request was decoded from JSON or a caller already has those bytes. The query
	// service forwards them to plugins instead of encoding the parsed tree again.
	// Requests built in code leave this nil and fall back to encoding.
	rawQueries [][]byte
}

// UnmarshalMetricRequest decodes a dashboard query request and keeps each query's
// original JSON. Null query elements stay nil, matching encoding/json's pointer rules.
func UnmarshalMetricRequest(b []byte) (MetricRequest, error) {
	var wire struct {
		From    string            `json:"from"`
		To      string            `json:"to"`
		Queries []json.RawMessage `json:"queries"`
		Debug   bool              `json:"debug"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&wire); err != nil {
		// An empty body is not a decode failure. web.Bind treats io.EOF the same way.
		if errors.Is(err, io.EOF) {
			return MetricRequest{}, nil
		}
		return MetricRequest{}, err
	}

	queries := make([]*simplejson.Json, len(wire.Queries))
	raw := make([][]byte, len(wire.Queries))
	for i, q := range wire.Queries {
		if bytes.Equal(bytes.TrimSpace(q), []byte("null")) {
			continue
		}
		parsed, err := simplejson.NewJson(q)
		if err != nil {
			return MetricRequest{}, fmt.Errorf("query %d: %w", i, err)
		}
		queries[i] = parsed
		raw[i] = q
	}
	return MetricRequest{
		From:       wire.From,
		To:         wire.To,
		Queries:    queries,
		Debug:      wire.Debug,
		rawQueries: raw,
	}, nil
}

// SetRawQueries attaches original query JSON parallel to Queries.
// A nil or mismatched slice is ignored and encoding falls back to the parsed tree.
func (mr *MetricRequest) SetRawQueries(raw [][]byte) {
	if len(raw) != len(mr.Queries) {
		return
	}
	mr.rawQueries = raw
}

// RawQuery returns the original JSON for query i, or nil when it is not available.
func (mr MetricRequest) RawQuery(i int) []byte {
	if i < 0 || i >= len(mr.rawQueries) {
		return nil
	}
	return mr.rawQueries[i]
}

func (mr *MetricRequest) GetUniqueDatasourceTypes() []string {
	dsTypes := make(map[string]bool)
	for _, query := range mr.Queries {
		if dsType, ok := query.Get("datasource").CheckGet("type"); ok {
			name := dsType.MustString()
			if _, ok := dsTypes[name]; !ok {
				dsTypes[name] = true
			}
		}
	}

	res := make([]string, 0, len(dsTypes))
	for dsType := range dsTypes {
		res = append(res, dsType)
	}

	return res
}

func (mr *MetricRequest) CloneWithQueries(queries []*simplejson.Json) MetricRequest {
	clone := MetricRequest{
		From:    mr.From,
		To:      mr.To,
		Queries: queries,
		Debug:   mr.Debug,
	}
	if len(mr.rawQueries) == 0 {
		return clone
	}
	raw := make([][]byte, len(queries))
	for i, q := range queries {
		if q == nil {
			continue
		}
		for j := range mr.Queries {
			if mr.Queries[j] == q && j < len(mr.rawQueries) {
				raw[i] = mr.rawQueries[j]
				break
			}
		}
	}
	clone.rawQueries = raw
	return clone
}

func GetGravatarUrl(cfg *setting.Cfg, text string) string {
	if cfg.DisableGravatar {
		return cfg.AppSubURL + "/public/img/user_profile.png"
	}

	if text == "" {
		return ""
	}

	hash, _ := GetGravatarHash(text)
	return fmt.Sprintf(cfg.AppSubURL+"/avatar/%x", hash)
}

func GetGravatarHash(text string) ([]byte, bool) {
	s := strings.TrimSpace(text)
	if s == "" {
		return make([]byte, 0), false
	}

	sum := sha256.Sum256([]byte(strings.ToLower(s)))
	return sum[:], true
}

func GetGravatarUrlWithDefault(cfg *setting.Cfg, text string, defaultText string) string {
	if text != "" {
		return GetGravatarUrl(cfg, text)
	}

	text = regNonAlphaNumeric.ReplaceAllString(defaultText, "") + "@localhost"

	return GetGravatarUrl(cfg, text)
}

func IsHiddenUser(userLogin string, signedInUser identity.Requester, cfg *setting.Cfg) bool {
	if userLogin == "" || signedInUser.GetIsGrafanaAdmin() || userLogin == signedInUser.GetLogin() {
		return false
	}

	if _, hidden := cfg.HiddenUsers[userLogin]; hidden {
		return true
	}

	return false
}
