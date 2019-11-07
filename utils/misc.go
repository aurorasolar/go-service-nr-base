package utils

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net"
	"time"
)

// Time in milliseconds since the Unix epoch
type AbsoluteTimeSec int64

func (at AbsoluteTimeSec) ToTime() time.Time {
	return time.Unix(int64(at), 0)
}

func (at AbsoluteTimeSec) ToUnix() int64 {
	return int64(at)
}

func FromTimeSec(tm time.Time) AbsoluteTimeSec {
	return AbsoluteTimeSec(tm.Unix())
}

func StaticClock(sec int64) func() time.Time {
	return func() time.Time {
		return time.Unix(sec, 0)
	}
}

func Use(_ interface{}) {}

func MakeRandomStr(numBytes int) string {
	bytesSlice := make([]byte, numBytes)
	_, err := rand.Read(bytesSlice)
	PanicIfF(err != nil, "failed to read random numbers: %s", err)
	return hex.EncodeToString(bytesSlice)
}

func PanicIfF(cond bool, msg string, args ...interface{}) {
	if cond {
		panic(fmt.Sprintf(msg, args...))
	}
}

// MemorySink implements zap.Sink by writing all messages to a buffer.
type MemorySink struct {
	bytes.Buffer
}
func (s *MemorySink) Close() error { return nil }
func (s *MemorySink) Sync() error  { return nil }

func NewMemorySinkLogger() (*MemorySink, *zap.Logger) {
	sink := &MemorySink{}
	config := zap.NewProductionEncoderConfig()
	config.TimeKey = ""
	core := zapcore.NewCore(zapcore.NewJSONEncoder(config), sink, zap.DebugLevel)
	logger := zap.New(core)
	return sink, logger
}


func GetFreeTcpPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}
