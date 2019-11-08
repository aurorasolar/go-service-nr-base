package visibility

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/aurorasolar/go-service-base/utils"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	newrelic "github.com/newrelic/go-agent"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"io"
	"strings"
	"testing"
)


type fakeSink struct {
	data map[string]MetricEntry
}

func (f *fakeSink) SubmitSegmentMetrics(seg *MetricsContext) {
	if f.data == nil {
		f.data = make(map[string]MetricEntry)
	}
	for n, v := range seg.Metrics {
		f.data[n] = *v
	}
}

func makeTestApp() newrelic.Application {
	cfg := newrelic.NewConfig("AppTest",
		"ffffffff56f2241ec3b97af491172aba267d1111")
	cfg.DistributedTracer.Enabled = true
	cfg.ServerlessMode.Enabled = true

	app, err := newrelic.NewApplication(cfg)
	utils.PanicIfF(err != nil, "failed to create an app")
	return app
}

func getMetrics(app newrelic.Application) map[string]interface{} {
	type serverlessWriter interface {
		ServerlessWrite(arn string, writer io.Writer)
	}

	sw := app.(serverlessWriter)
	var buf bytes.Buffer
	sw.ServerlessWrite("arn", &buf)

	obj := make([]interface{}, 0)
	err := json.Unmarshal(buf.Bytes(), &obj)
	utils.PanicIfF(err != nil, "failed to read")
	objData, err := base64.StdEncoding.DecodeString(obj[3].(string))
	utils.PanicIfF(err != nil, "failed to read")

	gzr, err := gzip.NewReader(bytes.NewReader(objData))
	utils.PanicIfF(err != nil, "failed to read")

	var b bytes.Buffer
	_, err = b.ReadFrom(gzr)
	utils.PanicIfF(err != nil, "failed to read")

	res := make(map[string]interface{})
	err = json.Unmarshal(b.Bytes(), &res)
	utils.PanicIfF(err != nil, "failed to read")

	return res
}

func getEvt(met map[string]interface{}, num int) map[string]interface{} {
	return met["analytic_event_data"].([]interface{})[2].
		([]interface{})[0].([]interface{})[num].(map[string]interface{})
}

func getErr(met map[string]interface{}, num int) map[string]interface{} {
	return met["error_event_data"].([]interface{})[2].
		([]interface{})[0].([]interface{})[num].(map[string]interface{})
}

func getStack(met map[string]interface{}, num int) []interface{} {
	err := met["error_data"].([]interface{})[1].
		([]interface{})[num].([]interface{})[4].(map[string]interface{})
	return err["stack_trace"].([]interface{})
}

func TestRunInstrumented(t *testing.T) {
	var seg newrelic.Transaction

	app := makeTestApp()

	err := RunInstrumented(context.Background(), "test1", app, NullSink, zap.NewNop(),
		func(c context.Context) error {
			seg = newrelic.FromContext(c)
			return fmt.Errorf("test err")
		})

	assert.NotNil(t, seg)
	assert.Error(t, err, "test err")

	met := getMetrics(app)
	evt := getEvt(met, 0)

	// Check that the metadata is set
	assert.Equal(t, "OtherTransaction/Go/test1", evt["name"])

	// Check that the error is recorded
	ex := getErr(met, 0)
	assert.Equal(t, "test err", ex["error.message"])
	assert.Equal(t, "*errors.errorString", ex["error.class"])
}

func TestRunInstrumentedPanic(t *testing.T) {
	app := makeTestApp()

	assert.Panics(t, func() {
		_ = RunInstrumented(context.Background(), "test1", app, NullSink, zap.NewNop(),
			func(c context.Context) error {
				panic("bad panic")
			})
	}, "bad panic")

	met := getMetrics(app)
	evt := getEvt(met, 0)

	// Check that the metadata is set
	assert.Equal(t, "OtherTransaction/Go/test1", evt["name"])

	// Check that the error is recorded
	ex := getErr(met, 0)
	assert.Equal(t, "bad panic", ex["error.message"])
	assert.Equal(t, "gopanic", ex["error.class"])

	stk := getStack(met, 0)
	// This is the line number of the panic() call above. Can break during refactoring.
	assert.Equal(t, float64(122), stk[0].(map[string]interface{})["line"])
	assert.True(t, strings.HasSuffix(stk[0].(map[string]interface{})["filepath"].(string),
		"runner_test.go"))
}

func TestSegmentWithMetrics(t *testing.T) {
	app := makeTestApp()

	sink := &fakeSink{}

	err := RunInstrumented(context.Background(), "test1", app, sink, zap.NewNop(),
		func(c context.Context) error {
			met := GetMetricsFromContext(c)
			met.AddCount("hellocount", 1)
			met.AddMetric("gigametric", 12, cloudwatch.StandardUnitGigabits)
			return nil
		})
	assert.NoError(t, err)

	// Metrics must be streamed!
	assert.Equal(t, float64(1), sink.data["hellocount"].Val)
	assert.Equal(t, float64(12), sink.data["gigametric"].Val)
	// Check that the transaction also has the correct metrics

	met := getMetrics(app)
	evt := getEvt(met, 1)

	// Check that the metadata is set
	assert.Equal(t, float64(1), evt["hellocount"])
	assert.Equal(t, "Count", evt["hellocountUnit"])

	assert.Equal(t, float64(12*1024*1024*1024), evt["gigametric"])
	assert.Equal(t, "Bits", evt["gigametricUnit"])
	assert.Equal(t, "Gigabits", evt["gigametricOrigUnit"])
}
