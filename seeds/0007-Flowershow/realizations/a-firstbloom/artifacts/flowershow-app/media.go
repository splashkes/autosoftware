package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/disintegration/imaging"
)

const (
	thumbnailMaxEdge     = 512
	thumbnailJPEGQuality = 80
	displayMaxEdge       = 1000
	displayJPEGQuality   = 86
	displayVariantSuffix = "_display.jpg"
	mediaVariantWorkers  = 6
	thumbnailContentType = "image/jpeg"
)

// preparePhotoForStorage normalizes image orientation and caps the stored
// display image to 1000px on its longest edge. Decode failures fall back to
// the original bytes so a bad client-side transform does not block upload.
func preparePhotoForStorage(data []byte, contentType string) ([]byte, string) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !strings.HasPrefix(contentType, "image/") {
		return data, contentType
	}
	prepared, err := generateDisplayPhoto(data, contentType)
	if err != nil {
		log.Printf("media: skip display normalization, decode failed: %v", err)
		return data, contentType
	}
	return prepared, thumbnailContentType
}

func generateDisplayPhoto(data []byte, contentType string) ([]byte, error) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !strings.HasPrefix(contentType, "image/") {
		return nil, errors.New("display image unavailable for non-image media")
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	img = fitImageMaxEdge(img, displayMaxEdge)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: displayJPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fitImageMaxEdge(img image.Image, maxEdge int) image.Image {
	if img == nil || maxEdge <= 0 {
		return img
	}
	bounds := img.Bounds()
	if bounds.Dx() <= maxEdge && bounds.Dy() <= maxEdge {
		return img
	}
	return imaging.Fit(img, maxEdge, maxEdge, imaging.Lanczos)
}

func displayPhotoFileName(name string) string {
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	if strings.TrimSpace(base) == "" {
		base = "image"
	}
	return base + ".jpg"
}

func mediaMaxEdge(media *Media) int {
	if media == nil {
		return 0
	}
	if media.Width > media.Height {
		return media.Width
	}
	return media.Height
}

func isPhotoMedia(media *Media) bool {
	if media == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(media.MediaType), "photo") {
		return true
	}
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(media.ContentType)), "image/")
}

func shouldServeDisplayVariant(media *Media) bool {
	if !isPhotoMedia(media) {
		return false
	}
	maxEdge := mediaMaxEdge(media)
	return maxEdge == 0 || maxEdge > displayMaxEdge
}

func encodeImageJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// generateThumbnail produces a max-512px-edge JPEG preview for image content.
// For non-image inputs or decode failure the helper returns (nil, nil) and the
// caller skips emitting a thumbnail (the upload still succeeds).
func generateThumbnail(data []byte, contentType string) ([]byte, error) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !strings.HasPrefix(contentType, "image/") {
		return nil, nil
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, err
	}
	thumb := fitImageMaxEdge(img, thumbnailMaxEdge)
	return encodeImageJPEG(thumb, thumbnailJPEGQuality)
}

// imageDimensions returns width and height of the supplied image bytes.
// Returns (0, 0) when the data cannot be decoded as an image.
func imageDimensions(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

const (
	maxPhotoSize = 20 << 20
	maxVideoSize = 50 << 20
)

var allowedPhotoTypes = map[string]string{
	"image/jpeg": "image/jpeg",
	"image/png":  "image/png",
	"image/webp": "image/webp",
}

var allowedVideoTypes = map[string]string{
	"video/mp4":       "video/mp4",
	"video/webm":      "video/webm",
	"video/quicktime": "video/quicktime",
}

var allowedPhotoExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".webp": "image/webp",
}

var allowedVideoExtensions = map[string]string{
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mov":  "video/quicktime",
}

type mediaStore interface {
	Store(ctx context.Context, entryID string, header *multipart.FileHeader) (*Media, error)
	GetURL(ctx context.Context, media *Media) (string, error)
	Open(ctx context.Context, media *Media) (io.ReadCloser, string, error)
	Delete(ctx context.Context, media *Media) error
}

type s3MediaStore struct {
	bucket       string
	region       string
	client       *s3.Client
	uploader     *manager.Uploader
	presigner    *s3.PresignClient
	variantCache *mediaVariantCache
}

type localMediaStore struct {
	dir string
}

type mediaVariantCache struct {
	mu       sync.Mutex
	inFlight map[string]*mediaVariantCall
	sem      chan struct{}
}

type mediaVariantCall struct {
	done chan struct{}
	err  error
}

func newMediaVariantCache(limit int) *mediaVariantCache {
	if limit <= 0 {
		limit = 1
	}
	return &mediaVariantCache{
		inFlight: make(map[string]*mediaVariantCall),
		sem:      make(chan struct{}, limit),
	}
}

func (c *mediaVariantCache) do(ctx context.Context, key string, fn func() error) error {
	if c == nil {
		return fn()
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("media variant key is required")
	}

	c.mu.Lock()
	if existing := c.inFlight[key]; existing != nil {
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-existing.done:
			return existing.err
		}
	}
	call := &mediaVariantCall{done: make(chan struct{})}
	c.inFlight[key] = call
	c.mu.Unlock()

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		c.finish(key, call, ctx.Err())
		return ctx.Err()
	}

	err := fn()
	c.finish(key, call, err)
	return err
}

func (c *mediaVariantCache) finish(key string, call *mediaVariantCall, err error) {
	c.mu.Lock()
	if c.inFlight[key] == call {
		delete(c.inFlight, key)
	}
	call.err = err
	close(call.done)
	c.mu.Unlock()
}

func newMediaStore() (mediaStore, error) {
	bucket := strings.TrimSpace(os.Getenv("AS_S3_BUCKET"))
	region := strings.TrimSpace(os.Getenv("AWS_REGION"))
	if bucket != "" && region != "" {
		cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
		if err != nil {
			return nil, err
		}
		creds, err := cfg.Credentials.Retrieve(context.Background())
		if err != nil {
			return nil, fmt.Errorf("s3 media store credentials unavailable for bucket %q in region %q: %w", bucket, region, err)
		}
		if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
			return nil, fmt.Errorf("s3 media store credentials unavailable for bucket %q in region %q: incomplete credential set", bucket, region)
		}
		client := s3.NewFromConfig(cfg)
		return &s3MediaStore{
			bucket:       bucket,
			region:       region,
			client:       client,
			uploader:     manager.NewUploader(client),
			presigner:    s3.NewPresignClient(client),
			variantCache: newMediaVariantCache(mediaVariantWorkers),
		}, nil
	}

	dir := strings.TrimSpace(os.Getenv("AS_MEDIA_DIR"))
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "flowershow-media")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &localMediaStore{dir: dir}, nil
}

func validateMediaSize(mediaType string, size int64) error {
	switch mediaType {
	case "video":
		if size > maxVideoSize {
			return fmt.Errorf("video exceeds %d MB limit", maxVideoSize>>20)
		}
	default:
		if size > maxPhotoSize {
			return fmt.Errorf("photo exceeds %d MB limit", maxPhotoSize>>20)
		}
	}
	return nil
}

func mediaContentType(header *multipart.FileHeader) string {
	contentType := header.Header.Get("Content-Type")
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if contentType != "" {
		return contentType
	}
	contentType = mime.TypeByExtension(strings.ToLower(filepath.Ext(header.Filename)))
	if contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func sanitizeFileName(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, " ", "-")
	if base == "." || base == "/" || base == "" {
		base = "upload"
	}
	return base
}

func canonicalMediaType(header *multipart.FileHeader) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType := mediaContentType(header)
	if ext == ".heic" || ext == ".heif" || contentType == "image/heic" || contentType == "image/heif" {
		return "", "", errors.New("HEIC/HEIF is not supported; use JPEG, PNG, or WebP")
	}
	if canonical, ok := allowedPhotoTypes[contentType]; ok {
		return "photo", canonical, nil
	}
	if canonical, ok := allowedVideoTypes[contentType]; ok {
		return "video", canonical, nil
	}
	if canonical, ok := allowedPhotoExtensions[ext]; ok {
		return "photo", canonical, nil
	}
	if canonical, ok := allowedVideoExtensions[ext]; ok {
		return "video", canonical, nil
	}
	if strings.HasPrefix(contentType, "image/") {
		return "", "", fmt.Errorf("unsupported photo type %q; use JPEG, PNG, or WebP", contentType)
	}
	if strings.HasPrefix(contentType, "video/") {
		return "", "", fmt.Errorf("unsupported video type %q; use MP4, WebM, or MOV", contentType)
	}
	return "", "", errors.New("unsupported media type; use JPEG, PNG, WebP, MP4, WebM, or MOV")
}

func (m *localMediaStore) Store(_ context.Context, entryID string, header *multipart.FileHeader) (*Media, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	mediaType, contentType, err := canonicalMediaType(header)
	if err != nil {
		return nil, err
	}
	if err := validateMediaSize(mediaType, header.Size); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	prepared, storedContentType := preparePhotoForStorage(body, contentType)
	contentType = storedContentType

	id := newID("media")
	name := sanitizeFileName(header.Filename)
	if mediaType == "photo" && strings.EqualFold(contentType, thumbnailContentType) {
		name = displayPhotoFileName(name)
	}
	path := filepath.Join(m.dir, id+"_"+name)
	if err := os.WriteFile(path, prepared, 0644); err != nil {
		return nil, err
	}

	width, height := imageDimensions(prepared)

	media := &Media{
		ID:          id,
		EntryID:     entryID,
		EntityKind:  "entry",
		MediaType:   mediaType,
		URL:         globalBasePath + "/media/" + id,
		ContentType: contentType,
		FileName:    name,
		StorageKey:  path,
		FileSize:    int64(len(prepared)),
		Width:       width,
		Height:      height,
		CreatedAt:   time.Now().UTC(),
	}

	if thumb, err := generateThumbnail(prepared, contentType); err == nil && len(thumb) > 0 {
		thumbPath := filepath.Join(m.dir, id+"_thumb.jpg")
		if werr := os.WriteFile(thumbPath, thumb, 0644); werr == nil {
			media.ThumbnailURL = globalBasePath + "/media/" + id + "?thumb=1"
		} else {
			log.Printf("media: thumbnail write failed for %s: %v", id, werr)
		}
	} else if err != nil {
		log.Printf("media: thumbnail generation failed for %s: %v", id, err)
	}

	return media, nil
}

func (m *localMediaStore) GetURL(_ context.Context, media *Media) (string, error) {
	return globalBasePath + "/media/" + media.ID, nil
}

func (m *localMediaStore) Open(_ context.Context, media *Media) (io.ReadCloser, string, error) {
	if media.StorageKey == "" {
		return nil, "", errors.New("missing media storage path")
	}
	f, err := os.Open(media.StorageKey)
	if err != nil {
		return nil, "", err
	}
	return f, media.ContentType, nil
}

func (m *localMediaStore) Delete(_ context.Context, media *Media) error {
	if media.StorageKey == "" {
		return nil
	}
	if err := os.Remove(media.StorageKey); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	thumbPath := localThumbPath(m.dir, media.ID)
	if err := os.Remove(thumbPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("media: thumbnail remove failed for %s: %v", media.ID, err)
	}
	return nil
}

// localThumbPath returns the on-disk path of a thumbnail given the storage
// directory and the media id.
func localThumbPath(dir, mediaID string) string {
	return filepath.Join(dir, mediaID+"_thumb.jpg")
}

// localThumbExists is a small helper used by handleMediaOpen to decide whether
// the thumbnail file is available on disk.
func localThumbExists(dir, mediaID string) bool {
	if _, err := os.Stat(localThumbPath(dir, mediaID)); err == nil {
		return true
	}
	return false
}

func (m *s3MediaStore) Store(ctx context.Context, entryID string, header *multipart.FileHeader) (*Media, error) {
	file, err := header.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	mediaType, contentType, err := canonicalMediaType(header)
	if err != nil {
		return nil, err
	}
	if err := validateMediaSize(mediaType, header.Size); err != nil {
		return nil, err
	}

	body, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	prepared, storedContentType := preparePhotoForStorage(body, contentType)
	contentType = storedContentType

	id := newID("media")
	name := sanitizeFileName(header.Filename)
	if mediaType == "photo" && strings.EqualFold(contentType, thumbnailContentType) {
		name = displayPhotoFileName(name)
	}
	key := fmt.Sprintf("entries/%s/%s_%s", entryID, id, name)
	_, err = m.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &m.bucket,
		Key:         &key,
		Body:        bytes.NewReader(prepared),
		ContentType: &contentType,
		ACL:         types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return nil, err
	}

	width, height := imageDimensions(prepared)

	media := &Media{
		ID:          id,
		EntryID:     entryID,
		EntityKind:  "entry",
		MediaType:   mediaType,
		URL:         globalBasePath + "/media/" + id,
		ContentType: contentType,
		FileName:    name,
		StorageKey:  key,
		FileSize:    int64(len(prepared)),
		Width:       width,
		Height:      height,
		CreatedAt:   time.Now().UTC(),
	}

	if thumb, terr := generateThumbnail(prepared, contentType); terr == nil && len(thumb) > 0 {
		thumbKey := key + "_thumb.jpg"
		thumbType := thumbnailContentType
		_, uerr := m.uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket:      &m.bucket,
			Key:         &thumbKey,
			Body:        bytes.NewReader(thumb),
			ContentType: &thumbType,
			ACL:         types.ObjectCannedACLPrivate,
		})
		if uerr == nil {
			media.ThumbnailURL = globalBasePath + "/media/" + id + "?thumb=1"
		} else {
			log.Printf("media: thumbnail upload failed for %s: %v", id, uerr)
		}
	} else if terr != nil {
		log.Printf("media: thumbnail generation failed for %s: %v", id, terr)
	}

	return media, nil
}

func (m *s3MediaStore) GetURL(ctx context.Context, media *Media) (string, error) {
	if shouldServeDisplayVariant(media) {
		if url, err := m.displayURL(ctx, media); err == nil {
			return url, nil
		} else {
			log.Printf("media: display variant unavailable for %s: %v", media.ID, err)
		}
	}
	return m.presignMediaObject(ctx, media.StorageKey, media.ContentType, media.FileName)
}

func (m *s3MediaStore) presignMediaObject(ctx context.Context, key, contentType, fileName string) (string, error) {
	out, err := m.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     &m.bucket,
		Key:                        &key,
		ResponseContentType:        &contentType,
		ResponseContentDisposition: awsString(`inline; filename="` + fileName + `"`),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = 15 * time.Minute
	})
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (m *s3MediaStore) Open(ctx context.Context, media *Media) (io.ReadCloser, string, error) {
	out, err := m.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &m.bucket,
		Key:    &media.StorageKey,
	})
	if err != nil {
		return nil, "", err
	}
	contentType := media.ContentType
	if out.ContentType != nil && *out.ContentType != "" {
		contentType = *out.ContentType
	}
	return out.Body, contentType, nil
}

func (m *s3MediaStore) thumbnailURL(ctx context.Context, media *Media) (string, error) {
	thumbKey := strings.TrimSpace(media.StorageKey) + "_thumb.jpg"
	if strings.TrimSpace(media.StorageKey) == "" {
		return "", errors.New("missing media storage key")
	}
	if mediaType := strings.TrimSpace(media.MediaType); mediaType != "" && !strings.EqualFold(mediaType, "photo") {
		return "", errors.New("thumbnail unavailable for non-photo media")
	}
	if _, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &m.bucket,
		Key:    &thumbKey,
	}); err != nil {
		if err := m.ensureGeneratedVariant(ctx, thumbKey, func() error {
			return m.generateAndStoreThumbnail(ctx, media, thumbKey)
		}); err != nil {
			return "", err
		}
	}
	return m.presignMediaObject(ctx, thumbKey, thumbnailContentType, displayPhotoFileName(media.FileName))
}

func (m *s3MediaStore) displayURL(ctx context.Context, media *Media) (string, error) {
	displayKey := strings.TrimSpace(media.StorageKey) + displayVariantSuffix
	if strings.TrimSpace(media.StorageKey) == "" {
		return "", errors.New("missing media storage key")
	}
	if _, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &m.bucket,
		Key:    &displayKey,
	}); err != nil {
		if err := m.ensureGeneratedVariant(ctx, displayKey, func() error {
			return m.generateAndStoreDisplay(ctx, media, displayKey)
		}); err != nil {
			return "", err
		}
	}
	return m.presignMediaObject(ctx, displayKey, thumbnailContentType, displayPhotoFileName(media.FileName))
}

func (m *s3MediaStore) ensureGeneratedVariant(ctx context.Context, key string, generate func() error) error {
	cache := m.variantCache
	if cache == nil {
		return generate()
	}
	return cache.do(ctx, key, func() error {
		if _, err := m.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: &m.bucket,
			Key:    &key,
		}); err == nil {
			return nil
		}
		return generate()
	})
}

func (m *s3MediaStore) generateAndStoreThumbnail(ctx context.Context, media *Media, thumbKey string) error {
	body, contentType, err := m.Open(ctx, media)
	if err != nil {
		return err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	thumb, err := generateThumbnail(data, contentType)
	if err != nil {
		return err
	}
	if len(thumb) == 0 {
		return errors.New("thumbnail unavailable for media")
	}
	thumbType := thumbnailContentType
	_, err = m.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &m.bucket,
		Key:         &thumbKey,
		Body:        bytes.NewReader(thumb),
		ContentType: &thumbType,
		ACL:         types.ObjectCannedACLPrivate,
	})
	return err
}

func (m *s3MediaStore) generateAndStoreDisplay(ctx context.Context, media *Media, displayKey string) error {
	body, contentType, err := m.Open(ctx, media)
	if err != nil {
		return err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	display, err := generateDisplayPhoto(data, contentType)
	if err != nil {
		return err
	}
	if len(display) == 0 {
		return errors.New("display image unavailable for media")
	}
	displayType := thumbnailContentType
	_, err = m.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &m.bucket,
		Key:         &displayKey,
		Body:        bytes.NewReader(display),
		ContentType: &displayType,
		ACL:         types.ObjectCannedACLPrivate,
	})
	return err
}

func (m *s3MediaStore) Delete(ctx context.Context, media *Media) error {
	_, err := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &m.bucket,
		Key:    &media.StorageKey,
	})
	if err != nil {
		return err
	}
	thumbKey := media.StorageKey + "_thumb.jpg"
	if _, derr := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &m.bucket,
		Key:    &thumbKey,
	}); derr != nil {
		log.Printf("media: s3 thumbnail delete failed for %s: %v", media.ID, derr)
	}
	displayKey := media.StorageKey + displayVariantSuffix
	if _, derr := m.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &m.bucket,
		Key:    &displayKey,
	}); derr != nil {
		log.Printf("media: s3 display variant delete failed for %s: %v", media.ID, derr)
	}
	return nil
}

func awsString(v string) *string { return &v }

func (a *app) handleMediaOpen(w http.ResponseWriter, r *http.Request) {
	mediaID := r.PathValue("mediaID")
	media, ok := a.store.mediaByID(mediaID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	wantThumb := strings.EqualFold(r.URL.Query().Get("thumb"), "1")
	if local, isLocal := a.media.(*localMediaStore); isLocal {
		if wantThumb && localThumbExists(local.dir, media.ID) {
			f, err := os.Open(localThumbPath(local.dir, media.ID))
			if err == nil {
				defer f.Close()
				w.Header().Set("Content-Type", thumbnailContentType)
				http.ServeContent(w, r, media.FileName+".thumb.jpg", media.CreatedAt, f)
				return
			}
			log.Printf("media: thumbnail open failed for %s: %v", media.ID, err)
		}
		body, contentType, err := a.media.Open(r.Context(), media)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer body.Close()
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		if media.FileName != "" {
			w.Header().Set("Content-Disposition", `inline; filename="`+media.FileName+`"`)
		}
		http.ServeContent(w, r, media.FileName, media.CreatedAt, readSeekNopCloser{body})
		return
	}

	if wantThumb {
		if s3store, ok := a.media.(*s3MediaStore); ok {
			if url, err := s3store.thumbnailURL(r.Context(), media); err == nil {
				http.Redirect(w, r, url, http.StatusTemporaryRedirect)
				return
			} else {
				log.Printf("media: thumbnail unavailable for %s: %v", media.ID, err)
			}
		}
	}

	url, err := a.media.GetURL(r.Context(), media)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

type readSeekNopCloser struct {
	io.ReadCloser
}

func (r readSeekNopCloser) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := r.ReadCloser.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, errors.New("media stream is not seekable")
}
