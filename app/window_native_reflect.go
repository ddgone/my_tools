package main

import (
	"context"
	"fmt"
	"reflect"
	"unsafe"
)

func frontendValueFromContext(ctx context.Context) (reflect.Value, error) {
	if ctx == nil {
		return reflect.Value{}, fmt.Errorf("应用尚未初始化")
	}

	frontend := ctx.Value("frontend")
	if frontend == nil {
		return reflect.Value{}, fmt.Errorf("缺少 Wails frontend 上下文")
	}

	value := reflect.ValueOf(frontend)
	if value.Kind() != reflect.Ptr || value.IsNil() {
		return reflect.Value{}, fmt.Errorf("Wails frontend 句柄无效")
	}

	return value, nil
}

func unsafeStructField(value reflect.Value, name string) (reflect.Value, error) {
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("目标不是结构体: %s", value.Kind())
	}

	field := value.FieldByName(name)
	if !field.IsValid() {
		return reflect.Value{}, fmt.Errorf("字段不存在: %s", name)
	}
	if !field.CanAddr() {
		return reflect.Value{}, fmt.Errorf("字段不可寻址: %s", name)
	}

	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem(), nil
}
