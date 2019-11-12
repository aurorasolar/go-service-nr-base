package visibility

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

//func setupServer(t *testing.T, logger *zap.Logger,
//	metrics *fakeSink, listener net.Listener) *echo.Echo {
//	// First, set up a minimal Echo server
//	e := echo.New()
//	e.HideBanner = true
//
//	// Insert the logging/tracing middleware
//	tmo := TracingAndMetricsOptions{
//		DebugMode:        true,
//		XrayName:         "HelloApp",
//		HostNameOverride: "http://somewhere.com",
//		EnableXray:       true,
//		Logger:           logger,
//		XRayLogLevel:     xraylog.LogLevelWarn,
//	}
//	e.Use(TracingAndLoggingMiddlewareHook(tmo))
//
//	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromData([]byte(schema))
//	assert.NoError(t, err)
//
//	e.Use(OapiRequestValidatorWithMetrics(swagger, "/api", nil, metrics))
//
//	e.GET("/api/run/*", func(ctx echo.Context) error {
//		c := ctx.Request().Context()
//		path := ctx.Request().URL.Path
//		CLS(c).Infof("From inside handler %s", path)
//
//		if strings.HasSuffix(path, "ok") {
//			GetMetricsFromContext(xray.GetSegment(c)).AddCount("Frob", 1)
//			return ctx.JSONBlob(http.StatusOK, []byte(`{"hello": "world"}`))
//		}
//		if strings.HasSuffix(path, "error") {
//			return echo.NewHTTPError(http.StatusConflict, "An error")
//		}
//		if strings.HasSuffix(path, "bad") {
//			return fmt.Errorf("bad error")
//		}
//
//		time.Sleep(200*time.Millisecond)
//		panic("unknown parameter")
//	})
//
//	go func() {
//		_ = e.Server.Serve(listener)
//	}()
//
//	return e
//}
//
//type sinkEmitter struct {
//	segments []*xray.Segment
//}
//
//func (s *sinkEmitter) Emit(seg *xray.Segment) {
//	s.segments = append(s.segments, seg)
//}
//
//func (s *sinkEmitter) RefreshEmitterWithAddress(raddr *net.UDPAddr) {
//}
//
//func TestEchoXray(t *testing.T) {
//	emitter := &sinkEmitter{}
//	_ = xray.Configure(xray.Config{
//		Emitter: emitter,
//	})
//
//	sink, logger := utils.NewMemorySinkLogger()
//	metricsSink := &fakeSink{}
//
//	xray.SetLogger(XrayLogAdapter(logger, xraylog.LogLevelDebug))
//
//	lstn, err := net.Listen("tcp", "[::]:9123")
//	assert.NoError(t, err)
//	//noinspection GoUnhandledErrorResult
//	defer lstn.Close()
//
//	e := setupServer(t, logger, metricsSink, lstn)
//	//noinspection GoUnhandledErrorResult
//	defer e.Shutdown(context.Background())
//
//	testOkCall(t, sink, emitter, metricsSink)
//	testRegularError(t, sink, emitter, metricsSink)
//	testBadError(t, sink, emitter, metricsSink)
//	testPanic(t, sink, emitter, metricsSink)
//
//	resp, err := http.Get("http://[::]:9123/api/unknown")
//	assert.NoError(t, err)
//	assert.Equal(t, 400, resp.StatusCode)
//
//	resp, err = http.Get("http://[::]:9123/api/run?param=123")
//	assert.NoError(t, err)
//	assert.Equal(t, 400, resp.StatusCode)
//}
//
//func testOkCall(t *testing.T, logSink *utils.MemorySink, segSink *sinkEmitter, metSink *fakeSink) {
//	req, err := http.NewRequest("GET", "http://[::]:9123/api/run/ok", nil)
//	assert.NoError(t, err)
//	req.Header.Set("x-amzn-trace-id",
//		"Root=1-5759e988-bd862e3fe1be46a994272793;Sampled=1")
//	resp, err := http.DefaultClient.Do(req)
//	assert.NoError(t, err)
//	assert.Equal(t, 200, resp.StatusCode)
//
//	assert.Equal(t, 1, len(segSink.segments))
//	seg := segSink.segments[0]
//	assert.Equal(t, "RunSomething", seg.Metadata[MetricsNamespaceName]["Operation"])
//	assert.Equal(t, "1-5759e988-bd862e3fe1be46a994272793", seg.TraceID)
//
//	// Check metrics
//	assert.Equal(t, 4, len(metSink.data))
//
//	assert.Equal(t, "Fault", *metSink.data[0].MetricName)
//	assert.Equal(t, 0., *metSink.data[0].Value)
//
//	assert.Equal(t, "Frob", *metSink.data[1].MetricName)
//	assert.Equal(t, 1., *metSink.data[1].Value)
//
//	assert.Equal(t, "Success", *metSink.data[2].MetricName)
//	assert.Equal(t, 1., *metSink.data[2].Value)
//
//	assert.Equal(t, "Time", *metSink.data[3].MetricName)
//
//	assert.True(t, strings.Contains(logSink.String(), `"msg":"Request finished"`))
//
//	segSink.segments = nil
//	metSink.data = nil
//	logSink.Reset()
//}
//
//func testRegularError(t *testing.T, logSink *utils.MemorySink,
//	segSink *sinkEmitter, metSink *fakeSink) {
//
//	req, err := http.NewRequest("GET", "http://[::]:9123/api/run/error", nil)
//	assert.NoError(t, err)
//	req.Header.Set("x-amzn-trace-id",
//		"Root=1-5759e988-bd862e3fe1be46a994272793;Sampled=?")
//	resp, err := http.DefaultClient.Do(req)
//	assert.NoError(t, err)
//	assert.Equal(t, 409, resp.StatusCode)
//
//	assert.Equal(t, 1, len(segSink.segments))
//	seg := segSink.segments[0]
//	assert.Equal(t, "1-5759e988-bd862e3fe1be46a994272793", seg.TraceID)
//	assert.Equal(t, true, seg.Sampled)
//
//	// Check metrics
//	assert.Equal(t, 3, len(metSink.data))
//	assert.Equal(t, "Fault", *metSink.data[0].MetricName)
//	assert.Equal(t, 0., *metSink.data[0].Value)
//	assert.Equal(t, "Success", *metSink.data[1].MetricName)
//	assert.Equal(t, 0., *metSink.data[1].Value)
//	assert.Equal(t, "Time", *metSink.data[2].MetricName)
//
//	logPrefix := `{"level":"info","logger":"HTTP","msg":"Starting request","RequestID":"1-5759e988-bd862e3fe1be46a994272793"}
//{"level":"info","logger":"HTTP","msg":"From inside handler /api/run/error","RequestID":"1-5759e988-bd862e3fe1be46a994272793"}
//{"level":"info","logger":"HTTP","msg":"Request error","RequestID":"1-5759e988-bd862e3fe1be46a994272793"`
//
//	assert.True(t, strings.HasPrefix(logSink.String(), logPrefix))
//
//	segSink.segments = nil
//	metSink.data = nil
//	logSink.Reset()
//}
//
//func testPanic(t *testing.T, logSink *utils.MemorySink,
//	segSink *sinkEmitter, metSink *fakeSink) {
//
//	resp, err := http.Get("http://[::]:9123/api/run/panic")
//	assert.NoError(t, err)
//	assert.Equal(t, 500, resp.StatusCode)
//
//	assert.Equal(t, 3, len(metSink.data))
//	assert.Equal(t, "Fault", *metSink.data[0].MetricName)
//	assert.Equal(t, 1., *metSink.data[0].Value)
//	assert.Equal(t, "Success", *metSink.data[1].MetricName)
//	assert.Equal(t, 0., *metSink.data[1].Value)
//	assert.Equal(t, "Time", *metSink.data[2].MetricName)
//	assert.True(t, *metSink.data[2].Value > 0.2)
//
//	assert.True(t, strings.Contains(logSink.String(), `"msg":"Request fault"`))
//	assert.True(t, strings.Contains(logSink.String(), "stacktrace"))
//
//	segSink.segments = nil
//	metSink.data = nil
//	logSink.Reset()
//}
//
//func testBadError(t *testing.T, logSink *utils.MemorySink,
//	segSink *sinkEmitter, metSink *fakeSink) {
//
//	resp, err := http.Get("http://[::]:9123/api/run/bad")
//	assert.NoError(t, err)
//	assert.Equal(t, 500, resp.StatusCode)
//
//	assert.Equal(t, 3, len(metSink.data))
//	assert.Equal(t, "Fault", *metSink.data[0].MetricName)
//	assert.Equal(t, 0., *metSink.data[0].Value)
//	assert.Equal(t, "Success", *metSink.data[1].MetricName)
//	assert.Equal(t, 0., *metSink.data[1].Value)
//	assert.Equal(t, "Time", *metSink.data[2].MetricName)
//
//	assert.True(t, strings.Contains(logSink.String(), `"msg":"Request error"`))
//
//	segSink.segments = nil
//	metSink.data = nil
//	logSink.Reset()
//}
