package utils

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"testing"
)

func TestTime(t *testing.T) {
	clock := StaticClock(10000)
	assert.Equal(t, int64(10000), clock().Unix())
	assert.Equal(t, 0, clock().Nanosecond())

	at := FromTimeSec(clock())
	assert.Equal(t, int64(10000), at.ToUnix())
	assert.Equal(t, int64(10000), at.ToTime().Unix())
}

func TestPanicIfF(t *testing.T) {
	PanicIfF(false, "hello")

	assert.PanicsWithValue(t, "bad panic error", func() {
		PanicIfF(true, "bad panic %s", fmt.Errorf("error"))
	})
}

func TestMakeRandomStr(t *testing.T) {
	assert.Equal(t, 20, len(MakeRandomStr(10)))
}

func TestMemorySinkLogger(t *testing.T) {
	sink, logger := NewMemorySinkLogger()
	logger.Info("hello, world")
	assert.Equal(t, "{\"level\":\"info\",\"msg\":\"hello, world\"}\n", sink.String())
	_ = sink.Close()
}

func TestGetFreeTcpPort(t *testing.T) {
	port, err := GetFreeTcpPort()
	assert.NoError(t, err)
	l1, err := net.Listen("tcp", fmt.Sprintf("[::0]:%d", port))
	assert.NoError(t, err)
	//noinspection GoUnhandledErrorResult
	defer l1.Close()

	port2, err := GetFreeTcpPort()
	assert.NoError(t, err)
	l2, err := net.Listen("tcp", fmt.Sprintf("[::0]:%d", port2))
	assert.NoError(t, err)
	//noinspection GoUnhandledErrorResult
	defer l2.Close()

	assert.NotEqual(t, port, port2)
}