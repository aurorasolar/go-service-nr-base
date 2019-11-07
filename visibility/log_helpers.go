package visibility

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"reflect"
	"runtime"
	"strconv"
	"strings"
)

const ZapLoggerEchoContextKey = "ZapLogger"

type loggerKey struct {
}

var loggerKeyVal = &loggerKey{}

func CL(ctx context.Context) *zap.Logger {
	value := ctx.Value(loggerKeyVal)
	if value == nil {
		panic("Trying to log from an unimbued context")
	}
	return value.(*zap.Logger)
}

func CLS(ctx context.Context) *zap.SugaredLogger {
	value := ctx.Value(loggerKeyVal)
	if value == nil {
		panic("Trying to log from an unimbued context")
	}
	return value.(*zap.Logger).Sugar()
}

func ImbueContext(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKeyVal, logger)
}

type ShortenedStackTrace struct {
	stack []uintptr
	msg   string
}

func NewShortenedStackTrace(skipFrames int, msg interface{}) *ShortenedStackTrace {
	// Register the stack trace inside the XRay segment
	s := make([]uintptr, 40)
	n := runtime.Callers(skipFrames, s)
	return &ShortenedStackTrace{stack: s[:n], msg: convertPanicMsg(msg)}
}

func convertPanicMsg(msg interface{}) string {
	if msg == nil {
		return "recovered from panic"
	}
	stringer, ok := msg.(fmt.Stringer)
	if ok {
		return stringer.String()
	}
	err, ok := msg.(error)
	if ok {
		return err.Error()
	}
	return reflect.ValueOf(msg).String()
}

func (s *ShortenedStackTrace) Error() string {
	return s.msg
}

func (s *ShortenedStackTrace) StackTrace() []uintptr {
	return s.stack
}

func (s *ShortenedStackTrace) JSONStack() interface{} {
	// Create the stack trace
	frames := runtime.CallersFrames(s.stack)

	stackElements := make([]map[string]string, 0, 20)
	// Note: On the last iteration, frames.Next() returns false, with a valid
	// frame, but we ignore this frame. The last frame is a a runtime frame which
	// adds noise, since it's only either runtime.main or runtime.goexit.
	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		path, line, label := s.parseFrame(frame)
		elem := make(map[string]string, 2)
		elem["fn"] = label
		elem["fl"] = path + ":" + strconv.Itoa(line)
		stackElements = append(stackElements, elem)
	}
	return stackElements
}

// The default stack trace contains the build environment full path as the
// first part of the file name. This adds no information to the stack trace,
// so process the stack trace to remove the build root path.
func (s *ShortenedStackTrace) parseFrame(frame runtime.Frame) (string, int, string) {
	path, line, label := frame.File, frame.Line, frame.Function

	// Strip GOPATH from path by counting the number of seperators in label & path
	// For example:
	//   GOPATH = /home/user
	//   path   = /home/user/src/pkg/sub/file.go
	//   label  = pkg/sub.Type.Method
	// We want to set path to:
	//    pkg/sub/file.go
	i := len(path)
	for n, g := 0, strings.Count(label, "/")+2; n < g; n++ {
		i = strings.LastIndex(path[:i], "/")
		if i == -1 {
			// Something went wrong and path has less separators than we expected
			// Abort and leave i as -1 to counteract the +1 below
			break
		}
	}
	path = path[i+1:] // Trim the initial /

	// Strip the path from the function name as it's already in the path
	label = label[strings.LastIndex(label, "/")+1:]
	// Likewise strip the package name
	label = label[strings.Index(label, ".")+1:]

	return path, line, label
}

func (s *ShortenedStackTrace) Field() zap.Field {
	return zap.Reflect("stacktrace", s.JSONStack())
}
