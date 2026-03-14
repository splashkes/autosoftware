package interactions

import (
	"context"
	"errors"
	"strings"
)

func (s *RuntimeService) UpsertSearchDocument(ctx context.Context, input UpsertSearchDocumentInput) (SearchDocument, error) {
	pool, err := expectReady(s)
	if err != nil {
		return SearchDocument{}, err
	}
	if strings.TrimSpace(input.SubjectKind) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return SearchDocument{}, errors.New("subject_kind and subject_id are required")
	}
	if strings.TrimSpace(input.DocumentID) == "" {
		input.DocumentID = newID("doc")
	}
	scope := statusOrDefault(input.Scope, "public")

	row := pool.QueryRow(ctx, `
		insert into runtime_search_documents (
		  document_id, subject_kind, subject_id, scope, language, title, summary,
		  body_text, facets, ranking, published_at, sort_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10::jsonb, $11, $12)
		on conflict (subject_kind, subject_id, scope)
		do update set
		  language = excluded.language,
		  title = excluded.title,
		  summary = excluded.summary,
		  body_text = excluded.body_text,
		  facets = excluded.facets,
		  ranking = excluded.ranking,
		  published_at = excluded.published_at,
		  sort_at = excluded.sort_at,
		  updated_at = now()
		returning document_id, subject_kind, subject_id, scope, language, title, summary,
		          body_text, facets::text, ranking::text, published_at, sort_at, updated_at
	`, input.DocumentID, input.SubjectKind, input.SubjectID, scope, nullString(input.Language),
		nullString(input.Title), nullString(input.Summary), input.BodyText, jsonBytes(input.Facets),
		jsonBytes(input.Ranking), nullTimeValue(input.PublishedAt), nullTimeValue(input.SortAt))

	item, err := scanSearchDocument(row)
	return item, wrapErr("upsert search document", err)
}

func (s *RuntimeService) SearchDocuments(ctx context.Context, input SearchDocumentsInput) ([]SearchDocument, error) {
	pool, err := expectReady(s)
	if err != nil {
		return nil, err
	}

	scope := statusOrDefault(input.Scope, "public")
	limit := clampLimit(input.Limit, 50, 200)
	query := "%" + strings.TrimSpace(input.Query) + "%"
	rows, err := pool.Query(ctx, `
		select document_id, subject_kind, subject_id, scope, language, title, summary,
		       body_text, facets::text, ranking::text, published_at, sort_at, updated_at
		from runtime_search_documents
		where scope = $1
		  and (
		    $2 = '%%'
		    or coalesce(title, '') ilike $2
		    or coalesce(summary, '') ilike $2
		    or body_text ilike $2
		  )
		order by coalesce(sort_at, updated_at) desc, updated_at desc
		limit $3
	`, scope, query, limit)
	if err != nil {
		return nil, wrapErr("search documents", err)
	}
	defer rows.Close()

	var items []SearchDocument
	for rows.Next() {
		item, err := scanSearchDocument(rows)
		if err != nil {
			return nil, wrapErr("scan search document", err)
		}
		items = append(items, item)
	}
	return items, wrapErr("search documents", rows.Err())
}
