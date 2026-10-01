package query

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/grafana/grafana-plugin-sdk-go/backend"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/localcache"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/services/datasources"
	fakeDatasources "github.com/grafana/grafana/pkg/services/datasources/fakes"
	"github.com/grafana/grafana/pkg/services/dsquerierclient"
	"github.com/grafana/grafana/pkg/services/pluginsintegration/pluginconfig"
	"github.com/grafana/grafana/pkg/services/pluginsintegration/plugincontext"
	"github.com/grafana/grafana/pkg/services/pluginsintegration/pluginstore"
	"github.com/grafana/grafana/pkg/services/user"
	"github.com/grafana/grafana/pkg/services/validations"
	"github.com/grafana/grafana/pkg/setting"
)

func BenchmarkParseMetricRequest(b *testing.B) {
	cases := []struct {
		name                  string
		queries               int
		datasources           int
		from, to              string
		perQueryTimeRange     bool
		supportLocalTimeRange bool
	}{
		{name: "single_ds_epoch_8q", queries: 8, datasources: 1, from: "1711929600000", to: "1711933200000"},
		{name: "single_ds_relative_8q", queries: 8, datasources: 1, from: "now-6h", to: "now"},
		{name: "single_ds_epoch_8q_local_tr", queries: 8, datasources: 1, from: "1711929600000", to: "1711933200000", perQueryTimeRange: true, supportLocalTimeRange: true},
		{name: "mixed_ds_epoch_12q", queries: 12, datasources: 3, from: "1711929600000", to: "1711933200000"},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchParse(b, tc.queries, tc.datasources, tc.from, tc.to, tc.perQueryTimeRange, tc.supportLocalTimeRange, true)
		})
		if tc.name == "single_ds_epoch_8q" {
			b.Run(tc.name+"_remarshal", func(b *testing.B) {
				benchParse(b, tc.queries, tc.datasources, tc.from, tc.to, tc.perQueryTimeRange, tc.supportLocalTimeRange, false)
			})
		}
	}
}

func BenchmarkQueryDataSingleDatasource(b *testing.B) {
	b.Run("reuse_original_json", func(b *testing.B) {
		benchQueryData(b, 8, 1, true)
	})
	b.Run("remarshal", func(b *testing.B) {
		benchQueryData(b, 8, 1, false)
	})
}

func benchParse(b *testing.B, queries, datasources int, from, to string, perQueryTimeRange, supportLocalTimeRange, keepRaw bool) {
	b.Helper()
	svc, user := benchQueryService(b, datasources)
	req := benchMetricRequest(b, queries, datasources, from, to, perQueryTimeRange, keepRaw)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parsed, err := svc.parseMetricRequest(ctx, user, false, req, supportLocalTimeRange)
		if err != nil {
			b.Fatal(err)
		}
		if len(parsed.getFlattenedQueries()) != queries {
			b.Fatal("query count")
		}
	}
}

func benchQueryData(b *testing.B, queries, datasources int, keepRaw bool) {
	b.Helper()
	svc, user := benchQueryService(b, datasources)
	req := benchMetricRequest(b, queries, datasources, "1711929600000", "1711933200000", false, keepRaw)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := svc.QueryData(ctx, user, false, req)
		if err != nil {
			b.Fatal(err)
		}
		if resp == nil {
			b.Fatal("nil response")
		}
	}
}

func BenchmarkQueryDataMixedDatasources(b *testing.B) {
	svc, user := benchQueryService(b, 3)
	req := benchMetricRequest(b, 12, 3, "1711929600000", "1711933200000", false, true)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := svc.QueryData(ctx, user, false, req)
		if err != nil {
			b.Fatal(err)
		}
		if resp == nil {
			b.Fatal("nil response")
		}
	}
}

func benchQueryService(b *testing.B, datasourceCount int) (*ServiceImpl, *user.SignedInUser) {
	b.Helper()
	dss := make([]*datasources.DataSource, 0, datasourceCount)
	pluginsList := make([]pluginstore.Plugin, 0, datasourceCount)
	for i := 0; i < datasourceCount; i++ {
		uid := fmt.Sprintf("ds-%d", i)
		dss = append(dss, &datasources.DataSource{
			ID:       int64(i + 1),
			UID:      uid,
			Type:     "prometheus",
			Name:     uid,
			JsonData: simplejson.NewFromAny(map[string]any{"timeInterval": "15s", "httpMethod": "POST"}),
		})
		pluginsList = append(pluginsList, pluginstore.Plugin{JSONData: plugins.JSONData{ID: "prometheus"}})
	}

	cfg := setting.NewCfg()
	pCtx := plugincontext.ProvideService(
		cfg,
		localcache.ProvideService(),
		pluginstore.NewFakePluginStore(pluginsList...),
		&fakeDatasources.FakeCacheService{DataSources: dss},
		&fakeDatasources.FakeDataSourceService{DataSources: dss},
		nil,
		pluginconfig.NewFakePluginRequestConfigProvider(),
	)

	svc := &ServiceImpl{
		cfg:                        cfg,
		dataSourceCache:            &fakeDatasources.FakeCacheService{DataSources: dss},
		dataSourceRequestValidator: validations.ProvideValidator(),
		pluginClient:               benchPluginClient{},
		pCtxProvider:               pCtx,
		log:                        log.New("query_data"),
		concurrentQueryLimit:       8,
		qsDatasourceClientBuilder:  dsquerierclient.NewNullQSDatasourceClientBuilder(),
	}
	return svc, &user.SignedInUser{OrgID: 1, Login: "bench", Name: "bench", Email: "bench@example.com"}
}

type benchPluginClient struct {
	plugins.Client
}

func (c benchPluginClient) QueryData(_ context.Context, _ *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	return backend.NewQueryDataResponse(), nil
}

func benchMetricRequest(b *testing.B, queries, datasources int, from, to string, perQueryTimeRange, keepRaw bool) dtos.MetricRequest {
	b.Helper()
	buf := benchMetricRequestJSON(b, queries, datasources, from, to, perQueryTimeRange)
	if keepRaw {
		req, err := dtos.UnmarshalMetricRequest(buf)
		if err != nil {
			b.Fatal(err)
		}
		return req
	}
	var req dtos.MetricRequest
	if err := json.Unmarshal(buf, &req); err != nil {
		b.Fatal(err)
	}
	return req
}

func benchMetricRequestJSON(b *testing.B, queries, datasources int, from, to string, perQueryTimeRange bool) []byte {
	b.Helper()
	payload := map[string]any{
		"from": from,
		"to":   to,
	}
	qs := make([]any, 0, queries)
	for i := 0; i < queries; i++ {
		uid := fmt.Sprintf("ds-%d", i%datasources)
		body := map[string]any{
			"refId":         string(rune('A' + i)),
			"datasource":    map[string]any{"type": "prometheus", "uid": uid},
			"expr":          `sum by (namespace, pod) (rate(container_cpu_usage_seconds_total{cluster="prod",namespace=~"observability|grafana"}[5m]))`,
			"intervalMs":    15000,
			"maxDataPoints": 876,
			"legendFormat":  "{{namespace}} {{pod}}",
			"editorMode":    "code",
			"range":         true,
			"exemplar":      false,
			"utcOffsetSec":  0,
		}
		if perQueryTimeRange {
			body["timeRange"] = map[string]any{"from": from, "to": to}
		}
		qs = append(qs, body)
	}
	payload["queries"] = qs
	buf, err := json.Marshal(payload)
	if err != nil {
		b.Fatal(err)
	}
	return buf
}
