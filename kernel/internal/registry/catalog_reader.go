package registry

import (
	"errors"
	"strings"
)

var ErrCatalogObjectNotFound = errors.New("registry catalog object not found")
var ErrCatalogSchemaNotFound = errors.New("registry catalog schema not found")
var ErrCatalogRealizationNotFound = errors.New("registry catalog realization not found")
var ErrCatalogCommandNotFound = errors.New("registry catalog command not found")
var ErrCatalogProjectionNotFound = errors.New("registry catalog projection not found")

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

func (r CatalogReader) ListRealizations(seedID, query string) ([]CatalogRealization, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	return FilterRealizations(catalog.Realizations, seedID, query), nil
}

func (r CatalogReader) GetRealization(reference string) (CatalogRealization, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return CatalogRealization{}, err
	}
	item, ok := GetRealization(catalog, reference)
	if !ok {
		return CatalogRealization{}, ErrCatalogRealizationNotFound
	}
	return item, nil
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

func (r CatalogReader) ListCommands(seedID, reference, query string) ([]CatalogCommand, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	return FilterCommands(catalog.Commands, seedID, reference, query), nil
}

func (r CatalogReader) GetCommand(reference, name string) (CatalogCommand, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return CatalogCommand{}, err
	}
	item, ok := GetCommand(catalog, reference, name)
	if !ok {
		return CatalogCommand{}, ErrCatalogCommandNotFound
	}
	return item, nil
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

func (r CatalogReader) ListProjections(seedID, reference, query string) ([]CatalogProjection, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	return FilterProjections(catalog.Projections, seedID, reference, query), nil
}

func (r CatalogReader) GetProjection(reference, name string) (CatalogProjection, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return CatalogProjection{}, err
	}
	item, ok := GetProjection(catalog, reference, name)
	if !ok {
		return CatalogProjection{}, ErrCatalogProjectionNotFound
	}
	return item, nil
}
