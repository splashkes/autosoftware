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

const publicRegistryBaseURL = "https://registry.autosoftware.app"

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
		return publicRegistryURL("/contracts/" + url.PathEscape(reference))
	}
	return publicRegistryURL("/contracts/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID))
}

func BrowseCommandPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return publicRegistryURL("/actions/" + url.PathEscape(reference) + "/" + url.PathEscape(name))
	}
	return publicRegistryURL("/actions/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name))
}

func BrowseProjectionPath(reference, name string) string {
	seedID, realizationID, ok := splitBrowseReference(reference)
	if !ok {
		return publicRegistryURL("/read-models/" + url.PathEscape(reference) + "/" + url.PathEscape(name))
	}
	return publicRegistryURL("/read-models/" + url.PathEscape(seedID) + "/" + url.PathEscape(realizationID) + "/" + url.PathEscape(name))
}

func BrowseObjectPath(seedID, kind string) string {
	return publicRegistryURL("/objects/" + url.PathEscape(seedID) + "/" + url.PathEscape(kind))
}

func BrowseSchemaPath(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return publicRegistryURL("/schemas")
	}
	pathPart, anchorPart, _ := strings.Cut(ref, "#")
	pathPart = strings.Trim(pathPart, "/")
	segments := strings.Split(pathPart, "/")
	escapedSegments := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		escapedSegments = append(escapedSegments, url.PathEscape(segment))
	}
	browsePath := "/schemas/" + strings.Join(escapedSegments, "/")
	if anchorPart = strings.TrimSpace(anchorPart); anchorPart != "" {
		browsePath += "/anchors/" + url.PathEscape(anchorPart)
	}
	return publicRegistryURL(browsePath)
}

func PermalinkBrowsePath(contentHash string) string {
	contentHash = strings.ToLower(strings.TrimSpace(contentHash))
	if contentHash == "" {
		return ""
	}
	return publicRegistryURL("/reg/" + contentHash)
}

func PermalinkResolvePath(canonicalURL, contentHash string) string {
	contentHash = strings.ToLower(strings.TrimSpace(contentHash))
	canonicalPath := registryPath(canonicalURL)
	if canonicalPath == "" || contentHash == "" {
		return ""
	}
	return publicRegistryURL("/@sha256-" + contentHash + canonicalPath)
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

func publicRegistryURL(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return publicRegistryBaseURL
	}
	if parsed, err := url.Parse(path); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.String()
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return publicRegistryBaseURL + path
}

func registryPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err == nil && parsed.Scheme != "" && parsed.Host != "" {
		path := parsed.EscapedPath()
		if path == "" {
			path = parsed.Path
		}
		if path == "" {
			return ""
		}
		if parsed.RawQuery != "" {
			path += "?" + parsed.RawQuery
		}
		return path
	}
	if !strings.HasPrefix(value, "/") {
		return ""
	}
	return value
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
