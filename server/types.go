package server

import (
	"github.com/mayocream/hath-go/pkg/hath"
)

// Config global config
type Config struct {
	hath.Config `mapstructure:",squash"`

	Debug    bool   `mapstructure:"debug"`
	LogLevel string `mapstructure:"log_level"`
}

// Hath ...
type Hath struct {
	*hath.Server
}

// NewHath ...
func NewHath(config Config) (*Hath, error) {
	initLogger(config)
	s, err := hath.NewServer(config.Config)
	if err != nil {
		return nil, err
	}
	return &Hath{s}, nil
}
