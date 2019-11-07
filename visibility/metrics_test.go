package visibility

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurorasolar/go-service-base/utils"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestMetricsContext(t *testing.T) {
	ctx := MakeMetricContext(context.Background(), "TestOp")
	mctx := GetMetricsFromContext(ctx)

	// Add metric can be called first, without a SetMetric
	mctx.AddMetric("zonk", 10, cloudwatch.StandardUnitCount)

	mctx.SetCount("count1", 11)
	mctx.SetCount("count1", 12) // Will override
	mctx.AddCount("count1", 2)

	mctx.SetMetric("speed", 123, cloudwatch.StandardUnitGigabits)
	mctx.AddMetric("speed", 2, cloudwatch.StandardUnitGigabits)

	mctx.SetDuration("duration", time.Millisecond*500)
	mctx.AddDuration("duration", time.Second*2)

	bench := mctx.Benchmark("delay")
	time.Sleep(500 * time.Millisecond)
	bench.Done()

	fs := &fakeSink{}
	fs.SubmitSegmentMetrics(mctx)
	metrics := fs.data

	assert.Equal(t, "TestOp", mctx.OpName)

	c1 := metrics["count1"]
	assert.Equal(t, cloudwatch.StandardUnitCount, c1.Unit)
	assert.Equal(t, 14.0, c1.Val)

	delay := metrics["delay"]
	assert.Equal(t, cloudwatch.StandardUnitSeconds, delay.Unit)
	assert.True(t, delay.Val > 0.5)

	duration := metrics["duration"]
	assert.Equal(t, cloudwatch.StandardUnitSeconds, duration.Unit)
	assert.Equal(t, 2.5, duration.Val)

	speed := metrics["speed"]
	assert.Equal(t, cloudwatch.StandardUnitGigabits, speed.Unit)
	assert.Equal(t, 125.0, speed.Val)

	zonk := metrics["zonk"]
	assert.Equal(t, cloudwatch.StandardUnitCount, zonk.Unit)
	assert.Equal(t, 10.0, zonk.Val)

	z1, zu := mctx.GetMetric("zonk")
	assert.Equal(t, 10.0, z1)
	assert.Equal(t, cloudwatch.StandardUnitCount, zu)
	assert.Equal(t, 10.0, mctx.GetMetricVal("zonk"))

	// Non-existing metric
	assert.Equal(t, 0.0, mctx.GetMetricVal("badbad"))

	mctx.Reset()
	mctx.Reset() // Idempotent

	z1, zu = mctx.GetMetric("zonk")
	assert.Equal(t, 0.0, z1)
	assert.Equal(t, cloudwatch.StandardUnitNone, zu)
	assert.Equal(t, 0.0, mctx.GetMetricVal("zonk"))
}

type fakeClient struct {
	data map[string]interface{}
}

func (f *fakeClient) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read the metrics data
	gzr, err := gzip.NewReader(req.Body)
	utils.PanicIfF(err != nil, "failed to read")

	var b bytes.Buffer
	_, err = b.ReadFrom(gzr)
	utils.PanicIfF(err != nil, "failed to read")

	var res []interface{}
	err = json.Unmarshal(b.Bytes(), &res)
	utils.PanicIfF(err != nil, "failed to read")
	f.data = res[0].(map[string]interface{})

	okRes := ioutil.NopCloser(strings.NewReader(""))
	return &http.Response{
		Status:     "OK",
		StatusCode: 200,
		Body:       okRes,
	}, nil
}

func TestMetricsSubmission(t *testing.T) {
	ctx := context.Background()
	ctx = MakeMetricContext(ctx, "TestCtx") // Save metrics into the context

	for i := 0; i < 17; i++ {
		mctx := GetMetricsFromContext(ctx)
		mctx.AddCount(fmt.Sprintf("count%d", i), 2)
		mctx.AddMetric(fmt.Sprintf("met%d", i), float64(i), cloudwatch.StandardUnitBytes)
	}

	fc := &fakeClient{}
	cli := &http.Client{Transport: fc}
	sink := NewMetricsSink("lic", "testApp", "Suffix", cli)
	sink.SubmitSegmentMetrics(GetMetricsFromContext(ctx))
	sink.Harvester.HarvestNow(ctx)

	// Check that the headers are set
	attrs := fc.data["common"].(map[string]interface{})["attributes"].(map[string]interface{})
	assert.Equal(t, "testApp", attrs["app.name"])
	assert.Equal(t, "Suffix", attrs["env"])

	mets := fc.data["metrics"].([]interface{})
	outer: for i := 0; i < 17; i++ {

		for _, m := range mets {
			mObj := m.(map[string]interface{})
			if mObj["name"] == fmt.Sprintf("TestCtx_count%d", i) {
				assert.Equal(t, "count", mObj["type"])
				assert.Equal(t, float64(2), mObj["value"])
				assert.Equal(t, "Count", mObj["attributes"].
					(map[string]interface{})["Unit"])
				break
			}
		}

		for _, m := range mets {
			mObj := m.(map[string]interface{})
			if mObj["name"] == fmt.Sprintf("TestCtx_met%d", i) {
				assert.Equal(t, "gauge", mObj["type"])
				assert.Equal(t, float64(i), mObj["value"])
				assert.Equal(t, "Bytes", mObj["attributes"].
					(map[string]interface{})["Unit"])
				continue outer
			}
		}
		assert.Fail(t, "failed to find a metric")
	}
}
