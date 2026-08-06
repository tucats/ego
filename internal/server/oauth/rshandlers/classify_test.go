package rshandlers

// Tests for classifyExchangeError, added as part of the REST-3 audit
// (docs/issues/REST-3.md, section 5.5): CallbackHandler used to answer a
// hardcoded 502 for every oauth.ExchangeCode/ExchangeCodePublic failure,
// discarding the distinctly typed error each failure mode already carries.

import (
	"net/http"
	"testing"

	"github.com/tucats/ego/internal/errors"
)

func TestClassifyExchangeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "Ego failed to build its own outbound request",
			err:  errors.New(errors.ErrOAuthTokenRequest),
			want: http.StatusInternalServerError,
		},
		{
			name: "IdP explicitly rejected the exchange",
			err:  errors.New(errors.ErrOAuthTokenError),
			want: http.StatusBadRequest,
		},
		{
			name: "network failure reaching the IdP",
			err:  errors.New(errors.ErrOAuthTokenPost),
			want: http.StatusBadGateway,
		},
		{
			name: "connection dropped reading the IdP's response",
			err:  errors.New(errors.ErrOAuthTokenRead),
			want: http.StatusBadGateway,
		},
		{
			name: "IdP response exceeded the size limit",
			err:  errors.New(errors.ErrOAuthTokenSizeLimit),
			want: http.StatusBadGateway,
		},
		{
			name: "IdP response body was not valid JSON",
			err:  errors.New(errors.ErrOAuthTokenParse),
			want: http.StatusBadGateway,
		},
		{
			name: "IdP responded with a non-200, non-error status",
			err:  errors.New(errors.ErrOAuthTokenHTTPStatus),
			want: http.StatusBadGateway,
		},
		{
			name: "IdP response had no access_token field",
			err:  errors.New(errors.ErrOAuthTokenNoToken),
			want: http.StatusBadGateway,
		},
		{
			name: "an error this function doesn't specifically know about",
			err:  errors.New(errors.ErrOIDCDiscoveryMissingToken),
			want: http.StatusBadGateway,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyExchangeError(tt.err); got != tt.want {
				t.Errorf("classifyExchangeError(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
