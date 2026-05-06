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
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/disintegration/imaging"
)

const (
	thumbnailMaxEdge      = 512
	thumbnailJPEGQuality  = 80
	rotatedJPEGQuality    = 90
	thumbnailContentType  = "image/jpeg"
)

// normalizeImageOrientation reads JPEG bytes that may carry EXIF orientation
// metadata, applies the rotation/flip, and returns re-encoded JPEG bytes with
// orientation reset. For non-JPEG inputs or any decode failure the original
// bytes are returned unchanged so an upload is never failed by this step.
func normalizeImageOrientation(data []byte, contentType string) ([]byte, error) {
	if !strings.EqualFold(strings.TrimSpace(contentType), "image/jpeg") {
		return data, nil
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		log.Printf("media: skip EXIF auto-rotate, decode failed: %v", err)
		return data, nil
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: rotatedJPEGQuality}); err != nil {
		log.Printf("media: skip EXIF auto-rotate, encode failed: %v", err)
		return data, nil
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
	thumb := imaging.Fit(img, thumbnailMaxEdge, thumbnailMaxEdge, imaging.Lanczos)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: thumbnailJPEGQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
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
	bucket    string
	region    string
	client    *s3.Client
	uploader  *manager.Uploader
	presigner *s3.PresignClient
}

type localMediaStore struct {
	dir string
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
			bucket:    bucket,
			region:    region,
			client:    client,
			uploader:  manager.NewUploader(client),
			presigner: s3.NewPresignClient(client),
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

	rotated, err := normalizeImageOrientation(body, contentType)
	if err != nil {
		log.Printf("media: orientation normalize error for %s: %v", header.Filename, err)
		rotated = body
	}

	id := newID("media")
	name := sanitizeFileName(header.Filename)
	path := filepath.Join(m.dir, id+"_"+name)
	if err := os.WriteFile(path, rotated, 0644); err != nil {
		return nil, err
	}

	width, height := imageDimensions(rotated)

	media := &Media{
		ID:          id,
		EntryID:     entryID,
		EntityKind:  "entry",
		MediaType:   mediaType,
		URL:         globalBasePath + "/media/" + id,
		ContentType: contentType,
		FileName:    name,
		StorageKey:  path,
		FileSize:    int64(len(rotated)),
		Width:       width,
		Height:      height,
		CreatedAt:   time.Now().UTC(),
	}

	if thumb, err := generateThumbnail(rotated, contentType); err == nil && len(thumb) > 0 {
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

	rotated, err := normalizeImageOrientation(body, contentType)
	if err != nil {
		log.Printf("media: orientation normalize error for %s: %v", header.Filename, err)
		rotated = body
	}

	id := newID("media")
	name := sanitizeFileName(header.Filename)
	key := fmt.Sprintf("entries/%s/%s_%s", entryID, id, name)
	_, err = m.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      &m.bucket,
		Key:         &key,
		Body:        bytes.NewReader(rotated),
		ContentType: &contentType,
		ACL:         types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return nil, err
	}

	width, height := imageDimensions(rotated)

	media := &Media{
		ID:          id,
		EntryID:     entryID,
		EntityKind:  "entry",
		MediaType:   mediaType,
		URL:         globalBasePath + "/media/" + id,
		ContentType: contentType,
		FileName:    name,
		StorageKey:  key,
		FileSize:    int64(len(rotated)),
		Width:       width,
		Height:      height,
		CreatedAt:   time.Now().UTC(),
	}

	if thumb, terr := generateThumbnail(rotated, contentType); terr == nil && len(thumb) > 0 {
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
	out, err := m.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     &m.bucket,
		Key:                        &media.StorageKey,
		ResponseContentType:        &media.ContentType,
		ResponseContentDisposition: awsString(`inline; filename="` + media.FileName + `"`),
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

	if wantThumb && media.ThumbnailURL != "" {
		if s3store, ok := a.media.(*s3MediaStore); ok {
			thumbKey := media.StorageKey + "_thumb.jpg"
			thumbType := thumbnailContentType
			out, err := s3store.presigner.PresignGetObject(r.Context(), &s3.GetObjectInput{
				Bucket:              &s3store.bucket,
				Key:                 &thumbKey,
				ResponseContentType: &thumbType,
			}, func(opts *s3.PresignOptions) {
				opts.Expires = 15 * time.Minute
			})
			if err == nil {
				http.Redirect(w, r, out.URL, http.StatusTemporaryRedirect)
				return
			}
			log.Printf("media: thumbnail presign failed for %s: %v", media.ID, err)
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
