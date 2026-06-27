package httpx

import "testing"

func TestV3ErrorCodesMapToExpectedHTTPStatuses(t *testing.T) {
	cases := map[string]int{
		CodeConnectorUnavailable:  502,
		CodeDBPoolExhausted:       503,
		CodeDBConnectTimeout:      504,
		CodeDBQueryTimeout:        408,
		CodeCircuitBreakerOpen:    503,
		CodeBackpressureLimit:     503,
		CodeQueryValidationFailed: 400,
		CodeScopeForbidden:        403,
		CodeNotFound:              404,
	}
	for code, want := range cases {
		if got := NewError(code, "x").HTTPStatus(); got != want {
			t.Fatalf("%s status = %d, want %d", code, got, want)
		}
	}
}
