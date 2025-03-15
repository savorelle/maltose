package mlog

import (
	"github.com/graingo/maltose/container/minstance"
)

const (
	DefaultName = "default"
)

var (
	instances = minstance.New()
)

// Instance returns the logger instance with the specified name.
func Instance(name ...string) *Logger {
	key := DefaultName
	if len(name) > 0 && name[0] != "" {
		key = name[0]
	}

	return instances.GetOrSetFunc(key, func() any {
		return New()
	}).(*Logger)
}
