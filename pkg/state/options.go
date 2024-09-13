package state

import (
	"github.com/go-logr/logr"
)

type Option func(*State)

func WithLogger(log logr.Logger) Option {
	return func(s *State) {
		s.log = log
	}
}
