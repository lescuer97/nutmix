package mint

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
)

func (m *Mint) SetupOidcService(ctx context.Context, url string) error {
	oidcClient, err := oidc.NewProvider(ctx, url)
	if err != nil {
		return err
	}

	m.OICDClient = oidcClient
	return nil
}
