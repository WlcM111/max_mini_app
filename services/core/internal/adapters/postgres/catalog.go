package postgres

import (
	"context"
	"fmt"
	"time"

	"vovremya/internal/platform/metrics"
	"vovremya/internal/platform/pgkit"
	"vovremya/services/core/internal/domain"
	"vovremya/services/core/internal/ports"
)

// CatalogRepo — чтение справочника при старте сервиса.
type CatalogRepo struct{ base }

// NewCatalogRepo создаёт репозиторий справочника.
func NewCatalogRepo(pool *pgkit.Pool, m *metrics.Registry) *CatalogRepo {
	return &CatalogRepo{base{pool: pool, metrics: m}}
}

var _ ports.CatalogRepo = (*CatalogRepo)(nil)

// Load читает справочник целиком.
func (r *CatalogRepo) Load(ctx context.Context) (*domain.Catalog, error) {
	started := time.Now()
	db := r.db(ctx)

	var categories []domain.BusinessCategory
	rows, err := db.Query(ctx, `SELECT code, title FROM core.business_categories ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("load categories: %w", err)
	}
	for rows.Next() {
		var c domain.BusinessCategory
		if err := rows.Scan(&c.Code, &c.Title); err != nil {
			rows.Close()
			return nil, err
		}
		categories = append(categories, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var regions []domain.Region
	rows, err = db.Query(ctx, `SELECT code, title, default_timezone FROM core.regions ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("load regions: %w", err)
	}
	for rows.Next() {
		var v domain.Region
		if err := rows.Scan(&v.Code, &v.Title, &v.DefaultTimezone); err != nil {
			rows.Close()
			return nil, err
		}
		regions = append(regions, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var features []domain.Feature
	rows, err = db.Query(ctx, `SELECT code, question, coalesce(hint, '') FROM core.features ORDER BY sort_order, code`)
	if err != nil {
		return nil, fmt.Errorf("load features: %w", err)
	}
	for rows.Next() {
		var v domain.Feature
		if err := rows.Scan(&v.Code, &v.Question, &v.Hint); err != nil {
			rows.Close()
			return nil, err
		}
		features = append(features, v)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	types := make([]domain.DocumentType, 0, 32)
	index := make(map[string]int, 32)
	rows, err = db.Query(ctx, `SELECT t.code, t.title, t.description, t.data_status, t.sort_order,
		s.title, s.url, s.checked_on
		FROM core.document_types t LEFT JOIN core.catalog_sources s ON s.id = t.source_id
		ORDER BY t.sort_order, t.code`)
	if err != nil {
		return nil, fmt.Errorf("load document types: %w", err)
	}
	for rows.Next() {
		var (
			t         domain.DocumentType
			title     *string
			url       *string
			checkedOn *time.Time
		)
		if err := rows.Scan(&t.Code, &t.Title, &t.Description, &t.DataStatus, &t.SortOrder,
			&title, &url, &checkedOn); err != nil {
			rows.Close()
			return nil, err
		}
		if title != nil && url != nil {
			src := domain.CatalogSource{Title: *title, URL: *url}
			if checkedOn != nil {
				src.CheckedOn = domain.FormatLocalDate(*checkedOn)
			}
			t.Source = &src
		}
		index[t.Code] = len(types)
		types = append(types, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `SELECT document_type_code, days_before FROM core.document_type_default_offsets
		ORDER BY document_type_code, days_before DESC`)
	if err != nil {
		return nil, fmt.Errorf("load default offsets: %w", err)
	}
	for rows.Next() {
		var (
			code string
			days int16
		)
		if err := rows.Scan(&code, &days); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := index[code]; ok {
			types[i].DefaultOffsets = append(types[i].DefaultOffsets, int(days))
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `SELECT document_type_code, step_text FROM core.document_type_renewal_steps
		ORDER BY document_type_code, step_no`)
	if err != nil {
		return nil, fmt.Errorf("load renewal steps: %w", err)
	}
	for rows.Next() {
		var code, text string
		if err := rows.Scan(&code, &text); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := index[code]; ok {
			types[i].RenewalSteps = append(types[i].RenewalSteps, text)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var rules []domain.ApplicabilityRule
	ruleIndex := make(map[int16]int)
	rows, err = db.Query(ctx, `SELECT id, document_type_code, coalesce(business_category_code, '')
		FROM core.applicability_rules ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load rules: %w", err)
	}
	for rows.Next() {
		var (
			id   int16
			rule domain.ApplicabilityRule
		)
		if err := rows.Scan(&id, &rule.DocumentTypeCode, &rule.BusinessCategoryCode); err != nil {
			rows.Close()
			return nil, err
		}
		ruleIndex[id] = len(rules)
		rules = append(rules, rule)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = db.Query(ctx, `SELECT rule_id, feature_code FROM core.applicability_rule_features ORDER BY rule_id`)
	if err != nil {
		return nil, fmt.Errorf("load rule features: %w", err)
	}
	for rows.Next() {
		var (
			id   int16
			code string
		)
		if err := rows.Scan(&id, &code); err != nil {
			rows.Close()
			return nil, err
		}
		if i, ok := ruleIndex[id]; ok {
			rules[i].FeatureCodes = append(rules[i].FeatureCodes, code)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	r.observe("catalog.load", started)
	return domain.NewCatalog(categories, regions, features, types, rules), nil
}
