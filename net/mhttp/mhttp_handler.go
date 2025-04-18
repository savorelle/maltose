package mhttp

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/graingo/maltose/errors/mcode"
	"github.com/graingo/maltose/errors/merror"
)

// HandlerFunc defines the basic handler function type.
type HandlerFunc func(*Request)

// handleRequest handles the request and returns the result.
func handleRequest(r *Request, method reflect.Method, val reflect.Value, req interface{}) error {
	// parameter binding
	if err := r.ShouldBind(req); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errMsgs []string
			for _, e := range validationErrors.Translate(r.GetTranslator()) {
				errMsgs = append(errMsgs, e)
			}
			if len(errMsgs) > 0 {
				return merror.NewCode(mcode.CodeValidationFailed, errMsgs[0])
			}
		}
		return err
	}

	// call method
	results := method.Func.Call([]reflect.Value{
		val,
		reflect.ValueOf(r.Request.Context()),
		reflect.ValueOf(req),
	})

	// handle return value
	if !results[1].IsNil() {
		return results[1].Interface().(error)
	}

	// set response to Request for middleware usage
	response := results[0].Interface()
	r.SetHandlerResponse(response)

	return nil
}

// checkMethodSignature checks the method signature.
func checkMethodSignature(typ reflect.Type) error {
	// check parameter number and return value number
	if typ.NumIn() != 3 || typ.NumOut() != 2 {
		return fmt.Errorf("invalid method signature, required: func(*Controller) (context.Context, *XxxReq) (*XxxRes, error)")
	}

	// check if the second parameter is context.Context
	if !typ.In(1).Implements(reflect.TypeOf((*context.Context)(nil)).Elem()) {
		return fmt.Errorf("first parameter should be context.Context")
	}

	// check if the third parameter is request parameter
	reqType := typ.In(2)
	if reqType.Kind() != reflect.Ptr {
		return fmt.Errorf("request parameter should be pointer type")
	}
	if !strings.HasSuffix(reqType.Elem().Name(), "Req") {
		return fmt.Errorf("request parameter should end with 'Req'")
	}

	// check if the first return value is response parameter
	resType := typ.Out(0)
	if resType.Kind() != reflect.Ptr {
		return fmt.Errorf("response parameter should be pointer type")
	}
	if !strings.HasSuffix(resType.Elem().Name(), "Res") {
		return fmt.Errorf("response parameter should end with 'Res'")
	}

	// check if the second return value is error
	if !typ.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return fmt.Errorf("second return value should be error")
	}

	return nil
}
