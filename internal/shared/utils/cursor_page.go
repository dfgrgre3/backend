package pagination

// CursorPage is the flat, standardized response shape for lazy-loaded /
// infinite-scroll lists:
//
//	{ "data": [...], "nextCursor": "...", "hasNextPage": true }
//
// It complements PaginatedResponse (which keeps cursors nested under
// "pagination"): high-traffic list endpoints expose this flat shape through
// their opt-in cursor mode (?v=2) so frontend SWR infinite hooks can consume
// it directly, while legacy consumers keep receiving the original response.
type CursorPage struct {
	Data        interface{} `json:"data"`
	NextCursor  string      `json:"nextCursor,omitempty"`
	HasNextPage bool        `json:"hasNextPage"`
}

// NewCursorPage builds a CursorPage from a fetched page slice and its paging
// metadata. Fetch Limit+1 rows and pass hasNext=len(rows)>limit so callers
// never need a separate COUNT query.
func NewCursorPage(data interface{}, nextCursor string, hasNext bool) CursorPage {
	return CursorPage{
		Data:        data,
		NextCursor:  nextCursor,
		HasNextPage: hasNext,
	}
}