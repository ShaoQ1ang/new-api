package entity

import "errors"

var ErrConfigVersionConflict = errors.New("AIGC model configuration version conflict")
var ErrIdempotencyConflict = errors.New("AIGC generation idempotency conflict")
var ErrRequestStateConflict = errors.New("AIGC generation state conflict")
var ErrGenerationNotFound = errors.New("AIGC generation not found")
