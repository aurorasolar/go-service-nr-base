package visibility

import (
	"context"
	"fmt"
	"github.com/aurorasolar/go-service-nr-base/utils"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/labstack/echo/v4"
	newrelic "github.com/newrelic/go-agent"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

const schema = `
{
  "openapi": "3.0.0",
  "info": {
    "version": "1.0.0",
    "title": "Test API"
  },
  "paths": {
    "/api/run/{res}": {
      "get": {
        "summary": "Run something",
        "operationId": "runSomething",
        "parameters": [
          {
            "name": "res",
            "in": "path",
            "required": true,
            "description": "Action type",
            "schema": {
              "type": "string"
            }
          }
        ],
        "responses": {
          "200": {
            "description": "OK"
          },
          "default": {
            "description": "unexpected error"
          }
        }
      }
    }
  }
}
`

func setupServer(t *testing.T, logger *zap.Logger, app newrelic.Application, metrics *fakeSink, listener net.Listener) *echo.Echo {
	// First, set up a minimal Echo server
	e := echo.New()
	e.HideBanner = true

	// Insert the logging/tracing middleware
	tmo := TracingAndMetricsOptions{
		DebugMode:        true,
		HostNameOverride: "http://somewhere.com",
		Logger:           logger,
		NrApp:            app,
	}
	e.Use(TracingAndLoggingMiddlewareHook(tmo))

	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData([]byte(schema))
	assert.NoError(t, err)

	e.Use(OapiRequestValidatorWithMetrics(swagger, "/api", nil, metrics))

	e.GET("/api/run/*", func(ctx echo.Context) error {
		c := ctx.Request().Context()
		path := ctx.Request().URL.Path
		CLS(c).Infof("From inside handler %s", path)

		if strings.HasSuffix(path, "ok") {
			GetMetricsFromContext(c).AddCount("Frob", 1)
			return ctx.JSONBlob(http.StatusOK, []byte(`{"hello": "world"}`))
		}
		if strings.HasSuffix(path, "error") {
			return echo.NewHTTPError(http.StatusConflict, "An error")
		}
		if strings.HasSuffix(path, "bad") {
			return fmt.Errorf("bad error")
		}

		time.Sleep(200*time.Millisecond)
		panic("unknown parameter")
	})

	go func() {
		_ = e.Server.Serve(listener)
	}()

	return e
}

func TestEchoTracing(t *testing.T) {
	app := makeTestApp()

	sink, logger := utils.NewMemorySinkLogger()
	metricsSink := &fakeSink{}

	lstn, err := net.Listen("tcp", "[::]:9123")
	assert.NoError(t, err)
	//noinspection GoUnhandledErrorResult
	defer lstn.Close()

	e := setupServer(t, logger, app, metricsSink, lstn)
	//noinspection GoUnhandledErrorResult
	defer e.Shutdown(context.Background())

	testOkCall(t, sink, app, metricsSink)
	testRegularError(t, sink, app, metricsSink)
	testBadError(t, sink, app, metricsSink)
	testPanic(t, sink, app, metricsSink)

	resp, err := http.Get("http://[::]:9123/api/unknown")
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)

	resp, err = http.Get("http://[::]:9123/api/run?param=123")
	assert.NoError(t, err)
	assert.Equal(t, 400, resp.StatusCode)
}

func testOkCall(t *testing.T, logSink *utils.MemorySink,
	app newrelic.Application, metSink *fakeSink) {

	req, err := http.NewRequest("GET", "http://[::]:9123/api/run/ok", nil)
	//req.Header.Set("x-amzn-trace-id",
	//	"Root=1-5759e988-bd862e3fe1be46a994272793;Sampled=?")
	assert.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Check metrics
	md := metSink.data
	assert.Equal(t, 4, len(md))

	assert.Equal(t, 0., md["Fault"].Val)
	assert.Equal(t, 1., md["Frob"].Val)
	assert.Equal(t, 1., md["Success"].Val)
	assert.True(t, md["Time"].Val > 0)

	assert.True(t, strings.Contains(logSink.String(), `"msg":"Request finished"`))

	md = nil
	logSink.Reset()
}

func testRegularError(t *testing.T, logSink *utils.MemorySink,
	app newrelic.Application, metSink *fakeSink) {

	req, err := http.NewRequest("GET", "http://[::]:9123/api/run/error", nil)
	assert.NoError(t, err)
	req.Header.Set("x-amzn-trace-id",
		"Root=1-5759e988-bd862e3fe1be46a994272793;Sampled=?")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)

	md := metSink.data
	//assert.Equal(t, 1, len(segSink.segments))
	//assert.Equal(t, "1-5759e988-bd862e3fe1be46a994272793", seg.TraceID)
	//assert.Equal(t, true, seg.Sampled)

	// Check metrics
	assert.Equal(t, 0., md["Fault"].Val)
	assert.Equal(t, 0., md["Success"].Val)
	assert.True(t, md["Time"].Val > 0)

//	logPrefix := `{"level":"info","logger":"HTTP","msg":"Starting request","RequestID":"1-5759e988-bd862e3fe1be46a994272793"}
//{"level":"info","logger":"HTTP","msg":"From inside handler /api/run/error","RequestID":"1-5759e988-bd862e3fe1be46a994272793"}
//{"level":"info","logger":"HTTP","msg":"Request error","RequestID":"1-5759e988-bd862e3fe1be46a994272793"`
//
//	assert.True(t, strings.HasPrefix(logSink.String(), logPrefix))

	metSink.data = nil
	logSink.Reset()
}

func testPanic(t *testing.T, logSink *utils.MemorySink,
	app newrelic.Application, metSink *fakeSink) {

	resp, err := http.Get("http://[::]:9123/api/run/panic")
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)

	assert.Equal(t, 3, len(metSink.data))
	md := metSink.data

	// Check metrics
	assert.Equal(t, 1., md["Fault"].Val)
	assert.Equal(t, 0., md["Success"].Val)
	assert.True(t, md["Time"].Val > 0.2)

	assert.True(t, strings.Contains(logSink.String(), `"msg":"Request fault"`))
	assert.True(t, strings.Contains(logSink.String(), "stacktrace"))

	metSink.data = nil
	logSink.Reset()
}

func testBadError(t *testing.T, logSink *utils.MemorySink,
	app newrelic.Application, metSink *fakeSink) {

	resp, err := http.Get("http://[::]:9123/api/run/bad")
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)

	// Check metrics
	md := metSink.data
	assert.Equal(t, 0., md["Fault"].Val)
	assert.Equal(t, 0., md["Success"].Val)
	assert.True(t, md["Time"].Val > 0.)

	assert.True(t, strings.Contains(logSink.String(), `"msg":"Request error"`))

	metSink.data = nil
	logSink.Reset()
}
