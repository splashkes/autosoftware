package main

import (
	"bytes"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/disintegration/imaging"
)

// buildJPEGWithEXIFOrientation6 returns a JPEG body whose pixel data is wider
// than tall but whose APP1 EXIF block declares Orientation = 6 (rotate 90 CW).
// On decode with imaging.AutoOrientation(true) the produced image should be
// taller than wide.
func buildJPEGWithEXIFOrientation6(t *testing.T) []byte {
	t.Helper()
	// 100x50 plain image (wider than tall before rotation).
	img := imaging.New(100, 50, color.RGBA{200, 100, 50, 255})
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	body := raw.Bytes()
	// JPEG starts with 0xFFD8. Insert an APP1 EXIF segment (TIFF little-endian)
	// declaring exactly one IFD0 entry: Orientation = 6.
	exif := buildEXIFOrientationSegment(6)
	if len(body) < 2 || body[0] != 0xFF || body[1] != 0xD8 {
		t.Fatalf("unexpected jpeg start bytes: %x", body[:2])
	}
	out := make([]byte, 0, len(body)+len(exif))
	out = append(out, 0xFF, 0xD8)
	out = append(out, exif...)
	out = append(out, body[2:]...)
	return out
}

// buildEXIFOrientationSegment returns bytes for a JPEG APP1 (FFE1) marker
// containing a minimal little-endian TIFF block whose only IFD0 entry sets
// the Orientation tag (0x0112) to the given value.
func buildEXIFOrientationSegment(orientation uint16) []byte {
	// Layout (counting from after FFE1+length):
	//   "Exif\0\0"                     6 bytes
	//   TIFF header "II*\0" + IFD0 off  8 bytes (offset = 8)
	//   IFD0 entry count (uint16)       2 bytes
	//   one entry (12 bytes)            12 bytes
	//   next IFD offset (0)             4 bytes
	// Total = 32 bytes.
	tiff := []byte{
		'I', 'I', 0x2A, 0x00, // little-endian, magic 0x002A
		0x08, 0x00, 0x00, 0x00, // IFD0 offset = 8
		0x01, 0x00, // 1 IFD entry
		0x12, 0x01, // tag 0x0112 = Orientation (LE)
		0x03, 0x00, // type 3 = SHORT
		0x01, 0x00, 0x00, 0x00, // count = 1
		byte(orientation), byte(orientation >> 8), 0x00, 0x00, // value
		0x00, 0x00, 0x00, 0x00, // next IFD = 0
	}
	exifPayload := append([]byte("Exif\x00\x00"), tiff...)
	segLen := uint16(len(exifPayload) + 2) // length field includes itself
	seg := []byte{0xFF, 0xE1, byte(segLen >> 8), byte(segLen)}
	seg = append(seg, exifPayload...)
	return seg
}

func TestMediaUploadRotatesEXIF(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))

	jpegBytes := buildJPEGWithEXIFOrientation6(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "rose.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(jpegBytes); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/admin/entries/entry_01/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}

	media := a.store.mediaByEntry("entry_01")
	if len(media) != 1 {
		t.Fatalf("expected 1 media record, got %d", len(media))
	}
	stored, err := os.ReadFile(media[0].StorageKey)
	if err != nil {
		t.Fatalf("read stored media: %v", err)
	}
	decoded, err := imaging.Decode(bytes.NewReader(stored))
	if err != nil {
		t.Fatalf("decode stored media: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() >= bounds.Dy() {
		t.Fatalf("expected rotated image (width < height), got %dx%d", bounds.Dx(), bounds.Dy())
	}
	if media[0].Width >= media[0].Height {
		t.Fatalf("expected stored width < height, got %dx%d", media[0].Width, media[0].Height)
	}
	if media[0].ThumbnailURL == "" {
		t.Fatal("expected thumbnail URL to be populated for image upload")
	}
}

func TestMediaUploadCapsDisplayPhotoAt1000Px(t *testing.T) {
	a := testApp()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /admin/entries/{entryID}/media", a.requireAdmin(a.handleMediaUpload))

	img := imaging.New(1600, 1200, color.RGBA{180, 70, 90, 255})
	var jpegBytes bytes.Buffer
	if err := jpeg.Encode(&jpegBytes, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("media", "large-rose.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(jpegBytes.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/admin/entries/entry_01/media", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	addAdminSession(t, a, req)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d: %s", w.Code, w.Body.String())
	}

	media := a.store.mediaByEntry("entry_01")
	if len(media) != 1 {
		t.Fatalf("expected 1 media record, got %d", len(media))
	}
	stored, err := os.ReadFile(media[0].StorageKey)
	if err != nil {
		t.Fatalf("read stored media: %v", err)
	}
	decoded, err := imaging.Decode(bytes.NewReader(stored))
	if err != nil {
		t.Fatalf("decode stored media: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 1000 || bounds.Dy() != 750 {
		t.Fatalf("expected 1600x1200 upload capped to 1000x750, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	if media[0].Width != 1000 || media[0].Height != 750 {
		t.Fatalf("expected media dimensions 1000x750, got %dx%d", media[0].Width, media[0].Height)
	}
	if media[0].ThumbnailURL == "" {
		t.Fatal("expected thumbnail URL to be populated for capped image upload")
	}
}

func TestMediaSetCoverIsExclusive(t *testing.T) {
	a := testApp()
	uploads := make([]*Media, 0, 3)
	for i := 0; i < 3; i++ {
		m, err := a.store.attachMedia(Media{
			EntryID:   "entry_01",
			MediaType: "photo",
			URL:       "https://example.com/p.jpg",
			FileName:  "p.jpg",
		})
		if err != nil {
			t.Fatalf("attach %d: %v", i, err)
		}
		uploads = append(uploads, m)
	}

	if err := a.store.setMediaCover(uploads[1].ID); err != nil {
		t.Fatalf("set cover #2: %v", err)
	}
	covers := coversFor(a, "entry_01")
	if len(covers) != 1 || covers[0].ID != uploads[1].ID {
		t.Fatalf("expected only #2 to be cover, got %v", coverIDs(covers))
	}

	if err := a.store.setMediaCover(uploads[2].ID); err != nil {
		t.Fatalf("set cover #3: %v", err)
	}
	covers = coversFor(a, "entry_01")
	if len(covers) != 1 || covers[0].ID != uploads[2].ID {
		t.Fatalf("expected only #3 to be cover after switch, got %v", coverIDs(covers))
	}

	for _, m := range a.store.mediaByEntry("entry_01") {
		if m.ID == uploads[1].ID && m.IsCover {
			t.Fatalf("media #2 should no longer be cover after cover switch")
		}
	}
}

func TestMediaAttachToClass(t *testing.T) {
	a := testApp()
	cls, ok := a.store.classByID("class_01")
	if !ok {
		t.Fatalf("expected demo class_01 to be present in seed data")
	}
	media, err := a.store.attachMediaToClass(AttachClassMediaInput{
		ClassID:   cls.ID,
		MediaType: "photo",
		URL:       "https://example.com/class-cover.jpg",
		FileName:  "class-cover.jpg",
	})
	if err != nil {
		t.Fatalf("attachMediaToClass: %v", err)
	}
	if media.EntityKind != "class" {
		t.Fatalf("expected EntityKind=class, got %q", media.EntityKind)
	}
	if media.EntryID != "" {
		t.Fatalf("expected empty EntryID for class-attached media, got %q", media.EntryID)
	}
	if media.ClassID != cls.ID {
		t.Fatalf("expected ClassID=%q, got %q", cls.ID, media.ClassID)
	}
	results := a.store.mediaByClass(cls.ID)
	found := false
	for _, m := range results {
		if m.ID == media.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mediaByClass should return the attached media id")
	}
	// Sanity: this class-attached row must NOT leak into a same-id entry list.
	for _, m := range a.store.mediaByEntry(cls.ID) {
		if m.ID == media.ID {
			t.Fatalf("class-attached media leaked into mediaByEntry result")
		}
	}
}

func coversFor(a *app, entryID string) []*Media {
	var out []*Media
	for _, m := range a.store.mediaByEntry(entryID) {
		if m.IsCover {
			out = append(out, m)
		}
	}
	return out
}

func coverIDs(items []*Media) []string {
	out := make([]string, 0, len(items))
	for _, m := range items {
		out = append(out, m.ID)
	}
	return out
}
