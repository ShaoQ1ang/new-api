package entity

import "errors"

var ErrConfigVersionConflict = errors.New("AIGC model configuration version conflict")
var ErrIdempotencyConflict = errors.New("AIGC generation idempotency conflict")
