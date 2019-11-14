// NewRelic and ZapLog integration with the Echo web framework. The middleware in this
// module makes sure that Echo requests and the Golang request context always have
// logging and XRay tracing set up.
//
// I realize that this module is _severely_ overloaded with functionality, but it's
// a trade-off, as we don't want to have deep caller stacks with loads of middleware.

package visibility

import (
	"fmt"
	. "github.com/aurorasolar/go-service-nr-base/utils"
	"github.com/labstack/echo/v4"
	newrelic "github.com/newrelic/go-agent"
	"go.uber.org/zap"
	"net/http"
	"reflect"
	"strconv"
	"time"
)

type TracingAndMetricsOptions struct {
	DebugMode  bool
	NrApp      newrelic.Application

	HostNameOverride string

	Logger *zap.Logger
}

func handlerPointer(handler echo.HandlerFunc) uintptr {
	return reflect.ValueOf(handler).Pointer()
}

func transactionName(c echo.Context) string {
	ptr := handlerPointer(c.Handler())
	if ptr == handlerPointer(echo.NotFoundHandler) {
		return "NotFoundHandler"
	}
	if ptr == handlerPointer(echo.MethodNotAllowedHandler) {
		return "MethodNotAllowedHandler"
	}
	return c.Path()
}

func (t *TracingAndMetricsOptions) Validate() {
	PanicIfF(t.Logger == nil, "logger was not set")
}

type traceAndLogMiddleware struct {
	next echo.HandlerFunc
	opts TracingAndMetricsOptions
}

// Store the original RequestIDs in annotations
func (z *traceAndLogMiddleware) moveRegularRequestIdToAnnotations(trans newrelic.Transaction,
	r *http.Request) {

	reqIdHeader := r.Header.Get(echo.HeaderXRequestID)
	if reqIdHeader != "" {
		_ = trans.AddAttribute("RequestId", reqIdHeader)
	}

	reqIdHeader = r.Header.Get("x-amzn-trace-id")
	if reqIdHeader != "" {
		_ = trans.AddAttribute("AmznTraceId", reqIdHeader)
	}
}

func (z *traceAndLogMiddleware) attachXrayTrace(c echo.Context) newrelic.Transaction {
	r := c.Request()

	trans := z.opts.NrApp.StartTransaction(transactionName(c),
		c.Response().Writer, c.Request())
	z.moveRegularRequestIdToAnnotations(trans, r)

	c.Response().Writer = trans

	// Add txn to c.Request().Context()
	ctx := newrelic.NewContext(c.Request().Context(), trans)
	c.SetRequest(c.Request().WithContext(ctx))

	// Synthesize the X-Request-ID header for anyone else in middleware
	// while storing the regular one in annotations
	c.Request().Header.Set(echo.HeaderXRequestID, trans.GetTraceMetadata().TraceID)

	return trans
}

func (z *traceAndLogMiddleware) createLogger(c echo.Context,
	trans newrelic.Transaction) *zap.Logger {

	fields := getLogLinkingMetadata(trans)

	// TODO: defend against large headers
	h := c.Request().Header
	reqIdHeader := h.Get("x-amzn-trace-id")
	if reqIdHeader != "" {
		fields = append(fields, zap.String("AmznTraceId", reqIdHeader))
	}
	reqIdHeader = h.Get(echo.HeaderXRequestID)
	if reqIdHeader != "" {
		fields = append(fields, zap.String("RequestId", reqIdHeader))
	}

	// Contextualize the logger
	logger := z.opts.Logger.Named("HTTP").With(fields...)

	// Save the logger in the Echo context in case middleware needs it
	c.Set(ZapLoggerEchoContextKey, logger.Sugar())
	c.SetLogger(NewLoggerWrapper(logger))

	// Save it in the request context as well
	contextWithLog := ImbueContext(c.Request().Context(), logger)
	c.SetRequest(c.Request().WithContext(contextWithLog))

	return logger
}

func (z *traceAndLogMiddleware) prepareCommonLogFields(c echo.Context,
	reqDuration time.Duration) []zap.Field {

	req := c.Request()
	res := c.Response()

	// Now log whatever happened
	bytesIn, err := strconv.ParseInt(req.Header.Get(echo.HeaderContentLength),
		10, 64)
	if err != nil {
		bytesIn = 0
	}
	p := req.URL.Path
	if p == "" {
		p = "/"
	}

	host := req.Host
	if z.opts.HostNameOverride != "" {
		host = z.opts.HostNameOverride
	}

	return []zap.Field{
		zap.String("path", p),
		zap.String("remote_ip", c.RealIP()),
		zap.String("host", host),
		zap.String("method", req.Method),
		zap.String("uri", req.RequestURI),
		zap.String("referer", req.Referer()),
		zap.String("user_agent", req.UserAgent()),
		zap.Int("status", res.Status),
		zap.Duration("latency", reqDuration),
		zap.String("latency_human", reqDuration.String()),
		zap.Int64("bytes_in", bytesIn),
		zap.Int64("bytes_out", res.Size),
	}
}

func (z *traceAndLogMiddleware) instrumentRequest(c echo.Context) error {
	origWriter := c.Response().Writer

	// Create the tracing context and attach it to the request
	trans := z.attachXrayTrace(c)
	defer func() { _ = trans.End() }()

	// Create a logger with this Request ID
	logger := z.createLogger(c, trans) // Mutates the c.Request().Context
	logger.Info("Starting request")

	start := time.Now()
	// Protect against panics
	defer func() {
		report := recover()
		if report == nil {
			return
		}

		// Register the stack trace inside the XRay segment
		stack := NewShortenedStackTrace(5, report)
		_ = trans.NoticeError(newrelic.Error{
			Message:    fmt.Sprintf("%v", report),
			Class:      "Panic",
			Stack:      stack.stack,
		})

		// Send the 500 error along the way...
		if !c.Response().Committed {
			if z.opts.DebugMode {
				// Send the stack trace along with the error in dev mode
				errMsg := make(map[string]interface{})
				errMsg["reason"] = stack.Error()
				errMsg["stacktrace"] = stack.JSONStack()
				c.Error(echo.NewHTTPError(http.StatusInternalServerError, errMsg))
			} else {
				c.Error(echo.ErrInternalServerError)
			}
		}

		ch := z.prepareCommonLogFields(c, time.Now().Sub(start))
		logger.Info("Request fault", append(ch, zap.Error(stack),
			stack.Field())...)
	}()

	// Actually process the request
	if err := z.next(c); err != nil {
		// Disable the response augmentation
		trans.SetWebResponse(nil)
		c.Response().Writer = origWriter

		// We have an error, process it
		c.Error(err)
		ch := z.prepareCommonLogFields(c, time.Now().Sub(start))
		httpErr, ok := err.(*echo.HTTPError)
		if ok {
			// HTTP errors contain a redundant code field
			logger.Info("Request error",
				append(ch, zap.Reflect("error", httpErr.Message))...)
			trans.WriteHeader(httpErr.Code)
		} else {
			logger.Info("Request error", append(ch, zap.Error(err))...)
			trans.WriteHeader(http.StatusInternalServerError)
		}
		return nil // Error is not propagated further
	}

	logger.Info("Request finished",
		z.prepareCommonLogFields(c, time.Now().Sub(start))...)

	return nil
}

// Insert middleware responsible for logging, metrics and tracing
func TracingAndLoggingMiddlewareHook(opts TracingAndMetricsOptions) echo.MiddlewareFunc {
	opts.Validate()

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		zlm := &traceAndLogMiddleware{
			opts: opts,
			next: next,
		}
		return zlm.instrumentRequest
	}
}
