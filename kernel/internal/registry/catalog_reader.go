package registry

import (
	"errors"
	"strings"
)

var ErrCatalogObjectNotFound = errors.New("registry catalog object not found")
var ErrCatalogSchemaNotFound = errors.New("registry catalog schema not found")

type CatalogReader struct {
	RepoRoot string
}

func NewCatalogReader(repoRoot string) CatalogReader {
	return CatalogReader{RepoRoot: strings.TrimSpace(repoRoot)}
}

func (r CatalogReader) Catalog() (Catalog, error) {
	return LoadCatalog(r.RepoRoot)
}

func (r CatalogReader) ListObjects(seedID, schemaRef, query string) ([]CatalogObject, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	return FilterObjects(catalog.Objects, seedID, schemaRef, query), nil
}

func (r CatalogReader) GetObject(seedID, kind string) (CatalogObject, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return CatalogObject{}, err
	}
	item, ok := GetObject(catalog, seedID, kind)
	if !ok {
		return CatalogObject{}, ErrCatalogObjectNotFound
	}
	return item, nil
}

func (r CatalogReader) ListSchemas(seedID, query string) ([]CatalogSchema, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	return FilterSchemas(catalog.Schemas, seedID, query), nil
}

func (r CatalogReader) GetSchema(ref string) (CatalogSchema, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return CatalogSchema{}, err
	}
	item, ok := GetSchema(catalog, ref)
	if !ok {
		return CatalogSchema{}, ErrCatalogSchemaNotFound
	}
	return item, nil
}
