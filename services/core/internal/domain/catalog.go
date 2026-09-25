package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
)

// CatalogSource — первоисточник сведений о типе документа.
type CatalogSource struct {
	Title     string
	URL       string
	CheckedOn string // YYYY-MM-DD либо пусто
}

// DocumentType — тип документа справочника.
type DocumentType struct {
	Code           string
	Title          string
	Description    string
	DataStatus     string // model | verified
	Source         *CatalogSource
	DefaultOffsets []int
	RenewalSteps   []string
	SortOrder      int
}

// BusinessCategory — вид деятельности.
type BusinessCategory struct {
	Code  string
	Title string
}

// Region — регион.
type Region struct {
	Code            string
	Title           string
	DefaultTimezone string
}

// Feature — признак организации.
type Feature struct {
	Code     string
	Question string
	Hint     string
}

// ApplicabilityRule — правило применимости типа документа.
type ApplicabilityRule struct {
	DocumentTypeCode     string
	BusinessCategoryCode string // пусто — любая категория
	FeatureCodes         []string
}

// Catalog — неизменяемый справочник, загружаемый при старте сервиса.
type Catalog struct {
	Version           string
	Categories        []BusinessCategory
	Regions           []Region
	Features          []Feature
	DocumentTypes     []DocumentType
	Rules             []ApplicabilityRule
	categoryIndex     map[string]BusinessCategory
	regionIndex       map[string]Region
	featureIndex      map[string]Feature
	documentTypeIndex map[string]DocumentType
}

// NewCatalog строит индексы и версию справочника (ETag).
func NewCatalog(categories []BusinessCategory, regions []Region, features []Feature,
	types []DocumentType, rules []ApplicabilityRule) *Catalog {
	c := &Catalog{
		Categories: categories, Regions: regions, Features: features,
		DocumentTypes: types, Rules: rules,
		categoryIndex:     make(map[string]BusinessCategory, len(categories)),
		regionIndex:       make(map[string]Region, len(regions)),
		featureIndex:      make(map[string]Feature, len(features)),
		documentTypeIndex: make(map[string]DocumentType, len(types)),
	}
	h := sha256.New()
	for _, v := range categories {
		c.categoryIndex[v.Code] = v
		_, _ = h.Write([]byte("c" + v.Code + v.Title))
	}
	for _, v := range regions {
		c.regionIndex[v.Code] = v
		_, _ = h.Write([]byte("r" + v.Code + v.Title + v.DefaultTimezone))
	}
	for _, v := range features {
		c.featureIndex[v.Code] = v
		_, _ = h.Write([]byte("f" + v.Code + v.Question + v.Hint))
	}
	for _, v := range types {
		c.documentTypeIndex[v.Code] = v
		_, _ = h.Write([]byte("t" + v.Code + v.Title + v.Description + v.DataStatus))
	}
	c.Version = hex.EncodeToString(h.Sum(nil))[:16]
	return c
}

// HasCategory сообщает, известен ли вид деятельности.
func (c *Catalog) HasCategory(code string) bool { _, ok := c.categoryIndex[code]; return ok }

// HasRegion сообщает, известен ли регион.
func (c *Catalog) HasRegion(code string) bool { _, ok := c.regionIndex[code]; return ok }

// HasFeature сообщает, известен ли признак.
func (c *Catalog) HasFeature(code string) bool { _, ok := c.featureIndex[code]; return ok }

// DocumentType возвращает тип документа по коду.
func (c *Catalog) DocumentType(code string) (DocumentType, bool) {
	t, ok := c.documentTypeIndex[code]
	return t, ok
}

// ETag возвращает значение заголовка ETag справочника.
func (c *Catalog) ETag() string { return `"` + c.Version + `"` }

// Applicable возвращает коды типов документов, подходящих организации:
// существует правило, у которого категория не задана или совпадает,
// и все признаки правила есть у организации (handoff §8).
func (c *Catalog) Applicable(categoryCode string, orgFeatures []string) []DocumentType {
	have := make(map[string]struct{}, len(orgFeatures))
	for _, f := range orgFeatures {
		have[f] = struct{}{}
	}
	matched := make(map[string]struct{})
	for _, rule := range c.Rules {
		if rule.BusinessCategoryCode != "" && rule.BusinessCategoryCode != categoryCode {
			continue
		}
		ok := true
		for _, f := range rule.FeatureCodes {
			if _, has := have[f]; !has {
				ok = false
				break
			}
		}
		if ok {
			matched[rule.DocumentTypeCode] = struct{}{}
		}
	}
	out := make([]DocumentType, 0, len(matched))
	for _, t := range c.DocumentTypes {
		if _, ok := matched[t.Code]; ok {
			out = append(out, t)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].SortOrder < out[j].SortOrder })
	return out
}

// DefaultOffsets возвращает отступы напоминаний по типу документа либо
// значения по умолчанию продукта (30, 7, 1).
func (c *Catalog) DefaultOffsets(typeCode string) []int {
	if t, ok := c.documentTypeIndex[typeCode]; ok && len(t.DefaultOffsets) > 0 {
		return append([]int(nil), t.DefaultOffsets...)
	}
	return []int{30, 7, 1}
}
