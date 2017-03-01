package config

import (
	"gopkg.in/go-playground/validator.v9"
)

var defaultValidator *validator.Validate

// Validator returns the default, singleton of Validate.
func Validator() *validator.Validate {
	return defaultValidator
}

func init() {
	defaultValidator = validator.New()
}
