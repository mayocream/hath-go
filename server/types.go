package server

import "github.com/mayocream/hath-go/pkg/hath"

// Config global config
type Config hath.Config

// Hath ...
type Hath struct {
	*hath.Server
}

// NewHath ...
func NewHath(config Config) (*Hath, error) {
	s, err := hath.NewServer(hath.Config(config))
	if err != nil {
		return nil, err
	}
	return &Hath{s}, nil
}
