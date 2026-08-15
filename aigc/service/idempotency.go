package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/aigc/entity"
	"github.com/QuantumNous/new-api/common"
)

var ErrIdempotencyConflict = entity.ErrIdempotencyConflict

type RequestStore interface {
	CreateOrGetRequest(ctx context.Context, request *entity.AigcRequest) (*entity.AigcRequest, bool, error)
	GetRequestByUserRequestID(ctx context.Context, userID int, requestID string) (*entity.AigcRequest, error)
}

func (service *IdempotencyService) Replay(ctx context.Context, userID int, requestID, requestDigest string) (*entity.AigcRequest, bool, error) {
	stored, err := service.requests.GetRequestByUserRequestID(ctx, userID, strings.TrimSpace(requestID))
	if errors.Is(err, entity.ErrGenerationNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if stored.RequestDigest != strings.TrimSpace(requestDigest) {
		return nil, false, ErrIdempotencyConflict
	}
	return stored, true, nil
}

type GenerationIDGenerator func() (string, error)

type BeginRequest struct {
	RequestID       string
	UserID          int
	TokenID         int
	GroupName       string
	PublicModelID   string
	UpstreamModelID string
	ModelType       string
	Mode            string
	ConfigVersion   int
	RequestDigest   string
	RequestJSON     string
	ExecutionJSON   string
}

type IdempotencyService struct {
	requests   RequestStore
	generateID GenerationIDGenerator
}

func NewIdempotencyService(requests RequestStore, generateID GenerationIDGenerator) *IdempotencyService {
	if generateID == nil {
		generateID = generateGenerationID
	}
	return &IdempotencyService{requests: requests, generateID: generateID}
}

func (service *IdempotencyService) Begin(ctx context.Context, input BeginRequest) (*entity.AigcRequest, bool, error) {
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.RequestDigest = strings.TrimSpace(input.RequestDigest)
	if input.RequestID == "" || input.UserID <= 0 || input.TokenID <= 0 {
		return nil, false, fmt.Errorf("request id, user id and token id are required")
	}
	if len(input.RequestDigest) != sha256.Size*2 {
		return nil, false, fmt.Errorf("request digest must be a SHA-256 hex digest")
	}
	generationID, err := service.generateID()
	if err != nil {
		return nil, false, err
	}
	request := &entity.AigcRequest{
		RequestID: input.RequestID, GenerationID: generationID, UserID: input.UserID, TokenID: input.TokenID,
		GroupName: strings.TrimSpace(input.GroupName), PublicModelID: strings.TrimSpace(input.PublicModelID),
		UpstreamModelID: strings.TrimSpace(input.UpstreamModelID), ModelType: strings.TrimSpace(input.ModelType),
		Mode: strings.TrimSpace(input.Mode), ConfigVersion: input.ConfigVersion, Status: entity.RequestStatusSubmitted,
		RequestDigest: input.RequestDigest, RequestJSON: input.RequestJSON,
		ExecutionJSON: input.ExecutionJSON,
	}
	stored, created, err := service.requests.CreateOrGetRequest(ctx, request)
	if err != nil {
		return nil, false, err
	}
	if !created && stored.RequestDigest != input.RequestDigest {
		return nil, false, ErrIdempotencyConflict
	}
	return stored, created, nil
}

func DigestGenerationRequest(value any) (string, error) {
	contents, err := common.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:]), nil
}

func generateGenerationID() (string, error) {
	key, err := common.GenerateRandomCharsKey(32)
	if err != nil {
		return "", err
	}
	return "aigc_gen_" + key, nil
}
