package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

var ErrHashLookupNotFound = errors.New("registry hash not found")

type ResourceLocator struct {
	ResourceKind string
	CanonicalURL string
	PermalinkURL string
	ContentHash  string
}

type HashLookupRecord struct {
	ContentHash  string
	ResourceKind string
	CanonicalURL string
	PermalinkURL string
}

func RealizationLocator(item CatalogRealization) ResourceLocator {
	return resourceLocator("realization", BrowseRealizationPath(item.Reference), item)
}

func CommandLocator(item CatalogCommand) ResourceLocator {
	return resourceLocator("command", BrowseCommandPath(item.Reference, item.Name), item)
}

func ProjectionLocator(item CatalogProjection) ResourceLocator {
	return resourceLocator("projection", BrowseProjectionPath(item.Reference, item.Name), item)
}

func ObjectLocator(item CatalogObject) ResourceLocator {
	return resourceLocator("object", BrowseObjectPath(item.SeedID, item.Kind), item)
}

func SchemaLocator(item CatalogSchema) ResourceLocator {
	return resourceLocator("schema", BrowseSchemaPath(item.Ref), item)
}

func ResourceContentHash(kind string, payload any) string {
	raw, err := json.Marshal(struct {
		Kind    string `json:"kind"`
		Payload any    `json:"payload"`
	}{
		Kind:    strings.TrimSpace(kind),
		Payload: payload,
	})
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func BrowseRealizationPath(reference string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/contracts/" + url.PathEscape(reference)
	}
	return "/contracts/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID)
}

func BrowseCommandPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/actions/" + url.PathEscape(reference) + "/" + url.PathEscape(name)
	}
	return "/actions/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name)
}

func BrowseProjectionPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return "/read-models/" + url.PathEscape(reference) + "/" + url.PathEscape(name)
	}
	return "/read-models/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name)
}

func BrowseObjectPath(seedID, kind string) string {
	return "/objects/" + url.PathEscape(seedID) + "/" + url.PathEscape(kind)
}

func BrowseSchemaPath(ref string) string {
	return "/schemas/" + url.PathEscape(strings.TrimSpace(ref))
}

func PermalinkBrowsePath(contentHash string) string {
	contentHash = strings.ToLower(strings.TrimSpace(contentHash))
	if contentHash == "" {
		return ""
	}
	return "/reg/" + contentHash
}

func PermalinkResolvePath(canonicalURL, contentHash string) string {
	canonicalURL = strings.TrimSpace(canonicalURL)
	contentHash = strings.ToLower(strings.TrimSpace(contentHash))
	if canonicalURL == "" || contentHash == "" {
		return ""
	}
	return "/@sha256-" + contentHash + canonicalURL
}

func IsSHA256Hex(value string) bool {
	if len(strings.TrimSpace(value)) != 64 {
		return false
	}
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

func resourceLocator(kind, canonicalURL string, payload any) ResourceLocator {
	contentHash := ResourceContentHash(kind, payload)
	return ResourceLocator{
		ResourceKind: strings.TrimSpace(kind),
		CanonicalURL: canonicalURL,
		PermalinkURL: PermalinkBrowsePath(contentHash),
		ContentHash:  contentHash,
	}
}

func splitBrowseReference(reference string) (string, string, bool) {
	reference = strings.Trim(strings.TrimSpace(reference), "/")
	if reference == "" {
		return "", "", false
	}
	parts := strings.Split(reference, "/")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}
