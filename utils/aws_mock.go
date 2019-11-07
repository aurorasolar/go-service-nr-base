package utils

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/defaults"
	"reflect"
)

type AwsMockHandler struct {
	handlers []reflect.Value
	functors []reflect.Value
}

// Create an AWS mocker to use with the AWS services, it returns an instrumented
// aws.Config that can be used to create AWS services.
// You can add as many individual request handlers as you need, as long as handlers
// correspond to the func(context.Context, <arg>)(<res>, error) format.
// E.g.:
// func(context.Context, *ec2.TerminateInstancesInput)(*ec2.TerminateInstancesOutput, error)
//
// You can also use a struct as the handler, in this case the AwsMockHandler will try
// to search for a method with a conforming signature.
func NewAwsMockHandler() *AwsMockHandler {
	return &AwsMockHandler{}
}

func (a *AwsMockHandler) AwsConfig() aws.Config {
	config := defaults.Config()
	config.Region = "us-mars-1"
	config.Credentials = aws.NewStaticCredentialsProvider("a", "b", "c")

	// Clear all the undesirable handlers
	clearAllHandlers(&config.Handlers)

	// Use the fake signer to override the request's handlers chain
	config.Handlers.Sign.PushFront(func(request *aws.Request) {
		clearAllHandlers(&request.Handlers)
		request.Handlers.Send.PushFront(a.requestHandler)
	})

	return config
}

func (a *AwsMockHandler) AddHandler(handlerObject interface {}) {
	handler := reflect.ValueOf(handlerObject)
	tp := handler.Type()

	if handler.Kind() == reflect.Func {
		PanicIfF(tp.NumOut() != 2 || tp.NumIn() != 2,
			"handler must have signature of func(context.Context, <arg>)(<res>, error)")
		a.functors = append(a.functors, handler)
	} else {
		PanicIfF(tp.NumMethod() == 0, "the handler must have invokable methods")
		a.handlers = append(a.handlers, handler)
	}
}

func (a *AwsMockHandler) requestHandler(request *aws.Request) {
	clearAllHandlers(&request.Handlers)

	request.Retryable = aws.Bool(false)

	res, err := a.invokeMethod(request.Context(), request.Params)
	if err != nil {
		request.Error = err
	} else {
		request.Data = res
	}
}

func clearAllHandlers(h *aws.Handlers) {
	h.Validate.Clear()
	h.Build.Clear()
	h.Sign.Clear()
	h.Send.Clear()
	h.AfterRetry.Clear()
	h.Unmarshal.Clear()
	h.UnmarshalError.Clear()
	h.UnmarshalMeta.Clear()
	h.ValidateResponse.Clear()
	h.Complete.Clear()
}

func (a *AwsMockHandler) invokeMethod(ctx context.Context,
	params interface{}) (interface{}, error) {

	for _, h := range a.handlers {
		for i := 0; i < h.NumMethod(); i++ {
			method := h.Method(i)

			matched, res, err := tryInvoke(ctx, params, method)
			if matched {
				return res, err
			}
		}
	}

	for _, f := range a.functors {
		matched, res, err := tryInvoke(ctx, params, f)
		if matched {
			return res, err
		}
	}

	panic("could not find a handler")
}

func tryInvoke(ctx context.Context, params interface{}, method reflect.Value) (
	bool, interface{}, error) {

	paramType := reflect.TypeOf(params)
	errorType := reflect.TypeOf((*error)(nil)).Elem()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()

	methodDesc := method.Type()
	if methodDesc.NumIn() != 2 || methodDesc.NumOut() != 2 {
		return false, nil, nil
	}

	if !contextType.ConvertibleTo(methodDesc.In(0)) {
		return false, nil, nil
	}
	if !paramType.ConvertibleTo(methodDesc.In(1)) {
		return false, nil, nil
	}
	if !methodDesc.Out(1).ConvertibleTo(errorType) {
		return false, nil, nil
	}

	// It's our target!
	res := method.Call([]reflect.Value{reflect.ValueOf(ctx),
		reflect.ValueOf(params)})

	if !res[1].IsNil() {
		return true, nil, res[1].Interface().(error)
	}

	return true, res[0].Interface(), nil
}
