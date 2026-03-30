package pagination

import "math"

type Params struct {
	Page  int
	Limit int
}

func New(page, limit int) Params {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	return Params{Page: page, Limit: limit}
}

func (p Params) Offset() int {
	return (p.Page - 1) * p.Limit
}

func Meta(total int64, params Params) map[string]interface{} {
	totalPages := int(math.Ceil(float64(total) / float64(params.Limit)))
	if total == 0 {
		totalPages = 0
	}

	return map[string]interface{}{
		"page":        params.Page,
		"limit":       params.Limit,
		"total":       total,
		"total_pages": totalPages,
	}
}
