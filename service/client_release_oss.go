package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

const defaultClientReleaseMaxBytes int64 = 500 << 20
const defaultClientReleaseUploadURLExpiresSeconds int64 = 3600
const clientReleaseTempDir = "_tmp"

type ClientReleaseUploadInput struct {
	Version  string
	Platform string
	Arch     string
	Channel  string
}

type ClientReleaseDirectUploadInput struct {
	ActorUserID int
	Version     string
	Platform    string
	Arch        string
	Channel     string
	FileName    string
	Size        int64
}

type ClientReleaseDirectUploadInitResult struct {
	FileName      string            `json:"fileName"`
	Object        string            `json:"object"`
	Size          int64             `json:"size"`
	ContentType   string            `json:"contentType"`
	UploadURL     string            `json:"uploadUrl"`
	UploadMethod  string            `json:"uploadMethod"`
	UploadHeaders map[string]string `json:"uploadHeaders"`
	UploadTicket  string            `json:"uploadTicket"`
	ExpiresAt     int64             `json:"expiresAt"`
}

type ClientReleaseUploadResult struct {
	FileName string `json:"fileName"`
	Object   string `json:"object"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
	SHA512   string `json:"sha512"`
}

type ClientReleasePromoteResult struct {
	Promoted    bool
	TempObject  string
	FinalObject string
}

type clientReleaseOSSConfig struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	Prefix          string
}

var clientReleaseContentTypes = map[string]string{
	".exe":      "application/vnd.microsoft.portable-executable",
	".msi":      "application/x-msi",
	".dmg":      "application/x-apple-diskimage",
	".pkg":      "application/octet-stream",
	".zip":      "application/zip",
	".appimage": "application/octet-stream",
	".deb":      "application/vnd.debian.binary-package",
	".rpm":      "application/x-rpm",
	".yml":      "text/yaml; charset=utf-8",
	".yaml":     "text/yaml; charset=utf-8",
}

type clientReleaseUploadTicket struct {
	ActorUserID int    `json:"actorUserId"`
	FileName    string `json:"fileName"`
	Object      string `json:"object"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
	ExpiresAt   int64  `json:"expiresAt"`
}

func InitClientReleaseDirectUpload(input ClientReleaseDirectUploadInput) (*ClientReleaseDirectUploadInitResult, error) {
	cfg := loadClientReleaseOSSConfig()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.FileName) == "" {
		return nil, errors.New("upload file name is required")
	}
	if input.ActorUserID <= 0 {
		return nil, errors.New("client release upload actor is invalid")
	}
	if input.Size <= 0 {
		return nil, errors.New("upload file is empty")
	}
	maxBytes := ClientReleaseMaxBytes()
	if input.Size > maxBytes {
		return nil, fmt.Errorf("client release package must be <= %d MB", maxBytes>>20)
	}
	contentType, err := clientReleaseContentType(input.FileName)
	if err != nil {
		return nil, err
	}
	uploadInput := ClientReleaseUploadInput{
		Version:  input.Version,
		Platform: input.Platform,
		Arch:     input.Arch,
		Channel:  input.Channel,
	}
	if err := normalizeClientReleaseUploadInput(&uploadInput); err != nil {
		return nil, err
	}
	if !model.IsAllowedClientReleaseUploadFile(uploadInput.Platform, input.FileName) {
		return nil, fmt.Errorf("client release file type is not supported for %s", uploadInput.Platform)
	}
	filename := clientReleaseGeneratedFileName(uploadInput, input.FileName)
	bucket, err := cfg.bucket()
	if err != nil {
		return nil, err
	}

	objectKey, err := cfg.tempObjectKey(filename)
	if err != nil {
		return nil, err
	}
	expires := clientReleaseUploadURLExpires()
	uploadURL, err := bucket.SignURL(objectKey, oss.HTTPPut, expires, oss.ContentType(contentType))
	if err != nil {
		return nil, err
	}
	ticket := clientReleaseUploadTicket{
		ActorUserID: input.ActorUserID,
		FileName:    filename,
		Object:      objectKey,
		Size:        input.Size,
		ContentType: contentType,
		ExpiresAt:   time.Now().Unix() + expires,
	}
	uploadTicket, err := signClientReleaseUploadTicket(ticket, cfg)
	if err != nil {
		return nil, err
	}

	return &ClientReleaseDirectUploadInitResult{
		FileName:     filename,
		Object:       objectKey,
		Size:         input.Size,
		ContentType:  contentType,
		UploadURL:    uploadURL,
		UploadMethod: string(oss.HTTPPut),
		UploadHeaders: map[string]string{
			"Content-Type": contentType,
		},
		UploadTicket: uploadTicket,
		ExpiresAt:    ticket.ExpiresAt,
	}, nil
}

func CompleteClientReleaseDirectUpload(uploadTicket string, actorUserID int) (*ClientReleaseUploadResult, error) {
	cfg := loadClientReleaseOSSConfig()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	ticket, err := parseClientReleaseUploadTicket(uploadTicket, cfg)
	if err != nil {
		return nil, err
	}
	if ticket.ExpiresAt <= time.Now().Unix() {
		return nil, errors.New("client release upload ticket has expired")
	}
	if actorUserID <= 0 || ticket.ActorUserID != actorUserID {
		return nil, errors.New("client release upload ticket does not belong to the current user")
	}
	if ticket.Object == "" || ticket.FileName == "" || ticket.Size <= 0 {
		return nil, errors.New("client release upload ticket is invalid")
	}
	if !cfg.isTempObjectKey(ticket.Object) {
		return nil, errors.New("client release upload ticket object is not temporary")
	}
	bucket, err := cfg.bucket()
	if err != nil {
		return nil, err
	}
	meta, err := bucket.GetObjectDetailedMeta(ticket.Object)
	if err != nil {
		return nil, err
	}
	if size, err := strconv.ParseInt(meta.Get("Content-Length"), 10, 64); err != nil || size != ticket.Size {
		_ = bucket.DeleteObject(ticket.Object)
		return nil, errors.New("client release uploaded object size does not match")
	}
	reader, err := bucket.GetObject(ticket.Object)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	sha256Hasher := sha256.New()
	sha512Hasher := sha512.New()
	size, err := io.Copy(io.MultiWriter(sha256Hasher, sha512Hasher), reader)
	if err != nil {
		return nil, err
	}
	if size != ticket.Size {
		_ = bucket.DeleteObject(ticket.Object)
		return nil, errors.New("client release uploaded object size does not match")
	}

	return &ClientReleaseUploadResult{
		FileName: ticket.FileName,
		Object:   ticket.Object,
		Size:     size,
		SHA256:   "sha256:" + hex.EncodeToString(sha256Hasher.Sum(nil)),
		SHA512:   base64.StdEncoding.EncodeToString(sha512Hasher.Sum(nil)),
	}, nil
}

func DiscardClientReleaseDirectUpload(uploadTicket string, actorUserID int) error {
	cfg := loadClientReleaseOSSConfig()
	if err := cfg.validate(); err != nil {
		return err
	}
	ticket, err := parseClientReleaseUploadTicket(uploadTicket, cfg)
	if err != nil {
		return err
	}
	if actorUserID <= 0 || ticket.ActorUserID != actorUserID {
		return errors.New("client release upload ticket does not belong to the current user")
	}
	if !cfg.isTempObjectKey(ticket.Object) {
		return errors.New("client release upload ticket object is not temporary")
	}
	return DeleteClientReleaseObject(ticket.Object)
}

func DeleteClientReleaseObject(objectKey string) error {
	cfg := loadClientReleaseOSSConfig()
	if err := cfg.validate(); err != nil {
		return err
	}
	objectKey, ok := cfg.managedObjectKey(objectKey)
	if !ok {
		return nil
	}
	bucket, err := cfg.bucket()
	if err != nil {
		return err
	}
	return bucket.DeleteObject(objectKey)
}

func PromoteClientReleaseObject(release *model.ClientRelease) (*ClientReleasePromoteResult, error) {
	return promoteClientReleaseAsset(
		release,
		&release.ObjectKey,
		release.FileName,
		&release.Size,
		&release.SHA256,
		&release.SHA512,
	)
}

func PromoteClientReleaseUpdaterObject(release *model.ClientRelease) (*ClientReleasePromoteResult, error) {
	if strings.TrimSpace(release.UpdaterObjectKey) == "" && strings.TrimSpace(release.UpdaterFileName) == "" {
		return &ClientReleasePromoteResult{}, nil
	}
	return promoteClientReleaseAsset(
		release,
		&release.UpdaterObjectKey,
		release.UpdaterFileName,
		&release.UpdaterSize,
		&release.UpdaterSHA256,
		&release.UpdaterSHA512,
	)
}

func promoteClientReleaseAsset(
	release *model.ClientRelease,
	objectKeyValue *string,
	fileName string,
	sizeValue *int64,
	sha256Value *string,
	sha512Value *string,
) (*ClientReleasePromoteResult, error) {
	cfg := loadClientReleaseOSSConfig()
	objectKey, ok := cfg.managedObjectKey(*objectKeyValue)
	if !ok {
		return nil, errors.New("client release OSS object is outside the managed prefix")
	}
	*objectKeyValue = objectKey
	if !cfg.isTempObjectKey(objectKey) {
		return &ClientReleasePromoteResult{}, nil
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	uploadInput := ClientReleaseUploadInput{
		Version:  release.Version,
		Platform: release.Platform,
		Arch:     release.Arch,
		Channel:  release.Channel,
	}
	if err := normalizeClientReleaseUploadInput(&uploadInput); err != nil {
		return nil, err
	}
	filename := cleanClientReleaseDownloadName(fileName)
	if filename == "" {
		return nil, errors.New("client release file name is required")
	}
	if expected := model.ClientReleaseExpectedFileName(uploadInput.Version, uploadInput.Platform, uploadInput.Arch, uploadInput.Channel, filename); filename != expected {
		return nil, errors.New("client release upload target does not match release metadata")
	}
	if cleanClientReleaseDownloadName(path.Base(objectKey)) != filename {
		return nil, errors.New("client release temporary object does not match uploaded file name")
	}
	finalObject := cfg.objectKey(uploadInput, filename)
	if cfg.isTempObjectKey(finalObject) || finalObject == objectKey {
		return nil, errors.New("client release final OSS object is invalid")
	}

	bucket, err := cfg.bucket()
	if err != nil {
		return nil, err
	}
	meta, err := bucket.GetObjectDetailedMeta(objectKey)
	if err != nil {
		return nil, err
	}
	objectSize, err := strconv.ParseInt(meta.Get("Content-Length"), 10, 64)
	if err != nil || objectSize <= 0 {
		return nil, errors.New("client release temporary object size is invalid")
	}
	etag := strings.TrimSpace(meta.Get("ETag"))
	if etag == "" {
		return nil, errors.New("client release temporary object etag is missing")
	}
	reader, err := bucket.GetObject(objectKey, oss.IfMatch(etag))
	if err != nil {
		return nil, err
	}
	sha256Hasher := sha256.New()
	sha512Hasher := sha512.New()
	hashedSize, hashErr := io.Copy(io.MultiWriter(sha256Hasher, sha512Hasher), reader)
	closeErr := reader.Close()
	if hashErr != nil {
		return nil, hashErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if hashedSize != objectSize {
		return nil, errors.New("client release temporary object changed while being verified")
	}
	verifiedSHA256 := "sha256:" + hex.EncodeToString(sha256Hasher.Sum(nil))
	verifiedSHA512 := base64.StdEncoding.EncodeToString(sha512Hasher.Sum(nil))
	if _, err := bucket.CopyObject(
		objectKey,
		finalObject,
		oss.CopySourceIfMatch(etag),
		oss.ForbidOverWrite(true),
	); err != nil {
		return nil, err
	}
	*objectKeyValue = finalObject
	*sizeValue = objectSize
	*sha256Value = verifiedSHA256
	*sha512Value = verifiedSHA512
	return &ClientReleasePromoteResult{
		Promoted:    true,
		TempObject:  objectKey,
		FinalObject: finalObject,
	}, nil
}

func SignClientReleaseURL(objectKey string, filename string) (string, error) {
	cfg := loadClientReleaseOSSConfig()
	if err := cfg.validate(); err != nil {
		return "", err
	}
	var ok bool
	objectKey, ok = cfg.managedObjectKey(objectKey)
	if !ok {
		return "", errors.New("client release oss object is required")
	}
	if cfg.isTempObjectKey(objectKey) {
		return "", errors.New("client release oss object is not finalized")
	}
	bucket, err := cfg.bucket()
	if err != nil {
		return "", err
	}
	expires := clientReleaseSignedURLExpires()
	options := []oss.Option{}
	if cleanName := cleanClientReleaseDownloadName(filename); cleanName != "" {
		options = append(options, oss.ResponseContentDisposition(
			fmt.Sprintf("attachment; filename=%q", cleanName),
		))
	}
	return bucket.SignURL(objectKey, oss.HTTPGet, expires, options...)
}

func normalizeClientReleaseUploadInput(input *ClientReleaseUploadInput) error {
	if strings.TrimSpace(input.Channel) == "" {
		return errors.New("client release channel is required")
	}
	input.Version = model.NormalizeClientReleaseVersion(input.Version)
	if err := model.ValidateClientReleaseVersion(input.Version); err != nil {
		return err
	}
	input.Platform = model.NormalizeClientReleasePlatform(input.Platform)
	input.Arch = model.NormalizeClientReleaseArch(input.Arch)
	input.Channel = model.NormalizeClientReleaseChannel(input.Channel)
	if !model.IsAllowedClientReleasePlatform(input.Platform) {
		return errors.New("client release platform must be windows, macos, or linux")
	}
	if !model.IsAllowedClientReleaseArch(input.Arch) {
		return errors.New("client release arch must be x64, arm64, ia32, or universal")
	}
	if !model.IsAllowedClientReleaseChannel(input.Channel) {
		return errors.New("client release channel must be stable or beta")
	}
	return nil
}

func ClientReleaseMaxBytes() int64 {
	value := strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_MAX_BYTES"))
	if value == "" {
		return defaultClientReleaseMaxBytes
	}
	bytes, err := strconv.ParseInt(value, 10, 64)
	if err != nil || bytes <= 0 {
		return defaultClientReleaseMaxBytes
	}
	return bytes
}

func loadClientReleaseOSSConfig() clientReleaseOSSConfig {
	return clientReleaseOSSConfig{
		Endpoint:        strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_ENDPOINT")),
		Bucket:          strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_BUCKET")),
		AccessKeyID:     strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_ACCESS_KEY_ID")),
		AccessKeySecret: strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_ACCESS_KEY_SECRET")),
		Prefix:          strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_PREFIX")),
	}
}

func (c clientReleaseOSSConfig) validate() error {
	if c.Endpoint == "" || c.Bucket == "" || c.AccessKeyID == "" || c.AccessKeySecret == "" {
		return errors.New("client release oss is not configured")
	}
	return nil
}

func (c clientReleaseOSSConfig) bucket() (*oss.Bucket, error) {
	client, err := oss.New(normalizeOSSEndpoint(c.Endpoint), c.AccessKeyID, c.AccessKeySecret)
	if err != nil {
		return nil, err
	}
	return client.Bucket(c.Bucket)
}

func (c clientReleaseOSSConfig) objectKey(input ClientReleaseUploadInput, filename string) string {
	channel := cleanObjectPart(input.Channel)
	if channel == "" {
		channel = "stable"
	}
	platform := cleanObjectPart(input.Platform)
	if platform == "" {
		platform = "unknown-platform"
	}
	arch := cleanObjectPart(input.Arch)
	if arch == "" {
		arch = "unknown-arch"
	}
	version := cleanObjectPart(input.Version)
	if version == "" {
		version = time.Now().UTC().Format("20060102150405")
	}
	name := cleanClientReleaseDownloadName(filename)
	if name == "" {
		name = "client-release"
	}
	stamp := time.Now().UTC().Format("20060102150405.000000000")
	return path.Join(c.basePrefix(), channel, platform, arch, version, fmt.Sprintf("%s-%s", stamp, name))
}

func (c clientReleaseOSSConfig) tempObjectKey(filename string) (string, error) {
	id, err := randomOSSObjectID()
	if err != nil {
		return "", err
	}
	name := cleanClientReleaseDownloadName(filename)
	if name == "" {
		name = "client-release"
	}
	return path.Join(c.basePrefix(), clientReleaseTempDir, id, name), nil
}

func (c clientReleaseOSSConfig) managedObjectKey(objectKey string) (string, bool) {
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	if objectKey == "" {
		return "", false
	}
	prefix := c.basePrefix()
	return objectKey, objectKey == prefix || strings.HasPrefix(objectKey, prefix+"/")
}

func (c clientReleaseOSSConfig) isTempObjectKey(objectKey string) bool {
	objectKey = strings.TrimLeft(strings.TrimSpace(objectKey), "/")
	tempPrefix := path.Join(c.basePrefix(), clientReleaseTempDir)
	return objectKey == tempPrefix || strings.HasPrefix(objectKey, tempPrefix+"/")
}

func (c clientReleaseOSSConfig) basePrefix() string {
	prefix := strings.Trim(strings.TrimSpace(c.Prefix), "/")
	if prefix == "" {
		prefix = "client-releases"
	}
	return prefix
}

func clientReleaseSignedURLExpires() int64 {
	value := strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_SIGNED_URL_EXPIRES_SECONDS"))
	if value == "" {
		return 600
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return 600
	}
	if seconds > 86400 {
		return 86400
	}
	return seconds
}

func clientReleaseUploadURLExpires() int64 {
	value := strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_UPLOAD_URL_EXPIRES_SECONDS"))
	if value == "" {
		return defaultClientReleaseUploadURLExpiresSeconds
	}
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil || seconds <= 0 {
		return defaultClientReleaseUploadURLExpiresSeconds
	}
	if seconds > 86400 {
		return 86400
	}
	return seconds
}

func signClientReleaseUploadTicket(ticket clientReleaseUploadTicket, cfg clientReleaseOSSConfig) (string, error) {
	payload, err := common.Marshal(ticket)
	if err != nil {
		return "", err
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	signature := signClientReleaseUploadPayload(encodedPayload, cfg)
	return encodedPayload + "." + signature, nil
}

func parseClientReleaseUploadTicket(value string, cfg clientReleaseOSSConfig) (*clientReleaseUploadTicket, error) {
	payload, signature, ok := strings.Cut(strings.TrimSpace(value), ".")
	if !ok || payload == "" || signature == "" {
		return nil, errors.New("client release upload ticket is invalid")
	}
	expected := signClientReleaseUploadPayload(payload, cfg)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return nil, errors.New("client release upload ticket is invalid")
	}
	data, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, errors.New("client release upload ticket is invalid")
	}
	var ticket clientReleaseUploadTicket
	if err := common.Unmarshal(data, &ticket); err != nil {
		return nil, errors.New("client release upload ticket is invalid")
	}
	ticket.Object = strings.TrimLeft(strings.TrimSpace(ticket.Object), "/")
	ticket.FileName = cleanClientReleaseDownloadName(ticket.FileName)
	ticket.ContentType = strings.TrimSpace(ticket.ContentType)
	return &ticket, nil
}

func signClientReleaseUploadPayload(payload string, cfg clientReleaseOSSConfig) string {
	secret := strings.TrimSpace(os.Getenv("CLIENT_RELEASE_OSS_UPLOAD_TICKET_SECRET"))
	if secret == "" {
		secret = cfg.AccessKeySecret
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func clientReleaseContentType(filename string) (string, error) {
	ext := clientReleaseFileExt(filename)
	contentType, ok := clientReleaseContentTypes[ext]
	if !ok {
		return "", errors.New("only exe, msi, dmg, pkg, zip, AppImage, deb, rpm, and yml files are supported")
	}
	return contentType, nil
}

func clientReleaseGeneratedFileName(input ClientReleaseUploadInput, filename string) string {
	return model.ClientReleaseExpectedFileName(
		input.Version,
		input.Platform,
		input.Arch,
		input.Channel,
		filename,
	)
}

func clientReleaseFileExt(filename string) string {
	return strings.ToLower(path.Ext(strings.ReplaceAll(filename, "\\", "/")))
}

func cleanClientReleaseDownloadName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	value = strings.Trim(value, ". ")
	if value == "." || value == "/" {
		return ""
	}
	if len(value) > 255 {
		return value[len(value)-255:]
	}
	return value
}

func randomOSSObjectID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}
