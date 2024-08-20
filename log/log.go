package log

import (
	"context"
	"unsafe"
)

type constant string

type Writer struct {
	format Format
}

func (w Writer) Report(ctx context.Context, err error) {
	w.format.Report(ctx, err)
}

func (w Writer) Record(ctx context.Context, subject any, event ...constant) {
	w.format.Record(ctx, subject, *(*[]string)(unsafe.Pointer(&event))...)
}

func New(format Format) Writer {
	return Writer{format}
}

type Format interface {
	Report(context.Context, error)
	Record(context.Context, any, ...string)
}
