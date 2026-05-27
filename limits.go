package shapeshifter

const (
	DefaultRequestBodyBytes  int64 = 65536
	DefaultResponseBodyBytes int64 = 1048576
)

type RouteKey struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Limits struct {
	RequestBodyBytes  int64
	ResponseBodyBytes int64
}
