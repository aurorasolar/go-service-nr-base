package visibility

import (
	"context"
	"fmt"
	"github.com/aurorasolar/go-service-nr-base/utils"
	newrelic "github.com/newrelic/go-agent"
	"go.uber.org/zap"
	"runtime"
)

const NewRelicAppContext = "NewRelicApp"

func AppFromContext(ctx context.Context) newrelic.Application {
	value := ctx.Value(NewRelicAppContext)
	res, ok := value.(newrelic.Application)
	utils.PanicIfF(!ok, "No New Relic attached")
	return res
}

func MakeAppContext(ctx context.Context, application newrelic.Application) context.Context {
	return context.WithValue(ctx, NewRelicAppContext, application)
}

// RunInstrumented() traces the provided synchronous function by
// beginning and closing a new subsegment around its execution.
// If the parent segment doesn't exist yet then a new top-level segment is created
func RunInstrumented(ctx context.Context, name string, app newrelic.Application,
	sink MetricsSink, logger *zap.Logger, fn func(context.Context) error) error {

	curTrans := newrelic.FromContext(ctx)
	var newTrans newrelic.Transaction
	if curTrans == nil {
		newTrans = app.StartTransaction(name, nil, nil)
	} else {
		newTrans = curTrans.NewGoroutine()
	}
	_ = newTrans.SetName(name)

	var err error
	defer func() {
		// (1) Close with the supplied error, either from the function
		// return or from the panic handler below.
		if err != nil {
			_ = newTrans.NoticeError(err)
		}
		_ = newTrans.End()
	}()

	defer func() {
		if p := recover(); p != nil {
			// OK, this is a serious COMEFROM-like trick here. In case of an
			// exception we modify the 'err' variable from the parent scope.
			// This in turn will be picked up by the deferred function (1).

			// Create an error with a nice stack trace
			stack := make([]uintptr, 40)
			n := runtime.Callers(3, stack)
			err = newrelic.Error{
				Message: fmt.Sprintf("%v", p),
				Class:   "gopanic",
				Stack:   stack[:n],
			}
			panic(p)
		}
	}()

	tid := newTrans.GetTraceMetadata().TraceID
	if tid == "" {
		tid = "AnonymousReq"
	}

	logger = logger.Named(name).With(zap.String("RequestID", tid))
	c := newrelic.NewContext(ctx, newTrans) // Create context with tracing attached
	c = ImbueContext(c, logger)             // Save logger into the context
	c = MakeMetricContext(c, name)          // Save metrics into the context

	met := GetMetricsFromContext(c)
	defer sink.SubmitSegmentMetrics(met)
	defer met.CopyToTransaction(newTrans)

	err = fn(c)

	return err
}
