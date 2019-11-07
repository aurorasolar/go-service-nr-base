package visibility

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aurorasolar/go-service-base/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"strings"
	"testing"
)


func TestContextLogging(t *testing.T) {
	ctx := context.Background()

	sink, logger := utils.NewMemorySinkLogger()

	imbued := ImbueContext(ctx, logger)
	CL(imbued).Info("Hello this is a test", zap.Int64("test", 123))
	CLS(imbued).Infof("Hello this is a test %d", 123)

	res := sink.String()
	splits := strings.Split(res, "\n")
	assert.True(t, strings.HasSuffix(splits[0],
		`"msg":"Hello this is a test","test":123}`))
	assert.True(t, strings.HasSuffix(splits[1],
		`"msg":"Hello this is a test 123"}`))
}

func TestNoLog(t *testing.T) {
	ctx := context.Background()
	assert.Panics(t, func() {
		CL(ctx)
	})
	assert.Panics(t, func() {
		CLS(ctx)
	})
}

func TestStackTraceStringer(t *testing.T) {
	trace := NewShortenedStackTrace(1, fmt.Errorf("test error"))
	assert.Equal(t, "test error", trace.Error())

	trace2 := NewShortenedStackTrace(1, "test error")
	assert.Equal(t, "test error", trace2.Error())

	trace3 := NewShortenedStackTrace(1, 123)
	assert.Equal(t, "<int Value>", trace3.Error())

	trace4 := NewShortenedStackTrace(1, nil)
	assert.Equal(t, "recovered from panic", trace4.Error())
}

type JsFrame struct {
	Fl string
	Fn string
}

func TestStackTrace(t *testing.T) {
	st := NewShortenedStackTrace(2, "Hello")
	jsStack := st.Field().Interface

	jsStr, err := json.Marshal(jsStack)
	assert.NoError(t, err)

	var res []JsFrame
	err = json.Unmarshal(jsStr, &res)
	assert.NoError(t, err)

	assert.Equal(t, "TestStackTrace", res[0].Fn)
	// This line must contain the line number of the NewShortenedStackTrace call,
	// might break during refactorings
	assert.True(t, strings.HasSuffix(res[0].Fl, "log_helpers_test.go:62"))
}
