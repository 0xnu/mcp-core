package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type TokenExtractor interface {
	ExtractToken(r *http.Request) (string, bool)
}

type BearerExtractor struct{}

func (e *BearerExtractor) ExtractToken(r *http.Request) (string, bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}

	token := strings.TrimPrefix(auth, "Bearer ")
	if token == "" {
		return "", false
	}

	return token, true
}

type APIKeyExtractor struct {
	Header string
	Param  string
}

func (e *APIKeyExtractor) ExtractToken(r *http.Request) (string, bool) {
	if e.Header != "" {
		token := r.Header.Get(e.Header)
		if token != "" {
			return token, true
		}
	}

	if e.Param != "" {
		token := r.URL.Query().Get(e.Param)
		if token != "" {
			return token, true
		}
	}

	return "", false
}

type MultiExtractor struct {
	extractors []TokenExtractor
}

func NewMultiExtractor(extractors ...TokenExtractor) *MultiExtractor {
	return &MultiExtractor{extractors: extractors}
}

func (me *MultiExtractor) ExtractToken(r *http.Request) (string, bool) {
	for _, e := range me.extractors {
		if token, ok := e.ExtractToken(r); ok {
			return token, true
		}
	}
	return "", false
}

type Transport struct {
	extractor TokenExtractor
	validator TokenValidator
}

type TokenValidator interface {
	Validate(token string) (bool, error)
}

type StaticTokenValidator struct {
	validTokens map[string]bool
}

func NewStaticTokenValidator(tokens []string) *StaticTokenValidator {
	valid := make(map[string]bool)
	for _, t := range tokens {
		valid[t] = true
	}
	return &StaticTokenValidator{validTokens: valid}
}

func (v *StaticTokenValidator) Validate(token string) (bool, error) {
	return v.validTokens[token], nil
}

type AllowAllValidator struct{}

func (v *AllowAllValidator) Validate(_ string) (bool, error) {
	return true, nil
}

func NewTransport(extractor TokenExtractor, validator TokenValidator) *Transport {
	return &Transport{
		extractor: extractor,
		validator: validator,
	}
}

func NewDefaultTransport(tokens []string) *Transport {
	extractor := NewMultiExtractor(
		&BearerExtractor{},
		&APIKeyExtractor{Header: "X-API-Key", Param: "api_key"},
	)

	var validator TokenValidator
	if len(tokens) > 0 {
		validator = NewStaticTokenValidator(tokens)
	} else {
		validator = &AllowAllValidator{}
	}

	return NewTransport(extractor, validator)
}

func (t *Transport) Authenticate(r *http.Request) error {
	token, ok := t.extractor.ExtractToken(r)
	if !ok {
		return errors.New("no authentication token found")
	}

	valid, err := t.validator.Validate(token)
	if err != nil {
		return fmt.Errorf("token validation error: %w", err)
	}

	if !valid {
		return errors.New("invalid authentication token")
	}

	return nil
}

func (t *Transport) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := t.Authenticate(r); err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
