package visibility

import (
	"context"
	"github.com/aurorasolar/go-service-base/utils"
	"github.com/newrelic/newrelic-telemetry-sdk-go/telemetry"
	"net/http"
)

type DefaultMetricsSink struct {
	Harvester *telemetry.Harvester
}

type MetricsSink interface {
	SubmitSegmentMetrics(met *MetricsContext)
}

type nullSink struct{
}
func (n *nullSink) SubmitSegmentMetrics(met *MetricsContext) {
}
var NullSink = &nullSink{}

func NewMetricsSink(nrLicenseKey, appName string, suffix string,
	client *http.Client) *DefaultMetricsSink {

	harv, err := telemetry.NewHarvester(
		telemetry.ConfigAPIKey(nrLicenseKey),
		func(c *telemetry.Config) {
			c.Client = client
		},
		telemetry.ConfigCommonAttributes(map[string]interface{}{
			"app.name": appName,
			"env":      suffix,
		}))
	utils.PanicIfF(err != nil, "Can't create the harvester")

	return &DefaultMetricsSink{Harvester: harv}
}

func (m *DefaultMetricsSink) SubmitSegmentMetrics(met *MetricsContext) {
	met.CopyToHarvester(m.Harvester)
}

func (m *DefaultMetricsSink) SendMetrics(ctx context.Context) error {
	m.Harvester.HarvestNow(ctx)
	return nil
}
