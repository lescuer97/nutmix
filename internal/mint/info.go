package mint

import (
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (m *Mint) Info() cashu.GetInfoResponse {
	contacts := []cashu.ContactInfo{}

	email := m.Config.EMAIL

	if len(email) > 0 {
		contacts = append(contacts, cashu.ContactInfo{
			Method: "email",
			Info:   email,
		})
	}

	nostr := m.Config.NOSTR

	if len(nostr) > 0 {
		contacts = append(contacts, cashu.ContactInfo{
			Method: "nostr",
			Info:   nostr,
		})
	}

	nuts := make(map[string]any)
	var baseNuts = []string{"1", "2", "3", "4", "5", "6"}

	var optionalNuts = []string{"7", "8", "9", "10", "11", "12", "17", "20"}

	if m.LightningBackend.ActiveMPP() {
		optionalNuts = append(optionalNuts, "15")
	}
	if m.Config.MINT_REQUIRE_AUTH {
		optionalNuts = append(optionalNuts, "21")
		optionalNuts = append(optionalNuts, "22")
	}

	for _, nut := range baseNuts {
		b := false

		switch nut {
		case "4":
			bolt11Method := cashu.SwapMintMethod{
				Method:    cashu.MethodBolt11,
				Unit:      cashu.Sat.String(),
				MinAmount: 0,
				MaxAmount: 0,
				Options:   nil,
				Commands:  nil,
			}

			if m.Config.PEG_IN_LIMIT_SATS != nil {
				bolt11Method.MaxAmount = *m.Config.PEG_IN_LIMIT_SATS
			}

			descriptionEnabled := m.LightningBackend.DescriptionSupport()
			bolt11Method.Options = &cashu.SwapMintMethodOptions{
				Description: &descriptionEnabled,
			}

			nuts[nut] = cashu.SwapMintInfo{
				Methods: &[]cashu.SwapMintMethod{
					bolt11Method,
				},
				Disabled:  &m.Config.PEG_OUT_ONLY,
				Supported: nil,
			}
		case "5":
			bolt11Method := cashu.SwapMintMethod{
				Method:    cashu.MethodBolt11,
				Unit:      cashu.Sat.String(),
				MinAmount: 0,
				MaxAmount: 0,
				Options:   nil,
				Commands:  nil,
			}

			if m.Config.PEG_OUT_LIMIT_SATS != nil {
				bolt11Method.MaxAmount = *m.Config.PEG_OUT_LIMIT_SATS
			}

			nuts[nut] = cashu.SwapMintInfo{
				Methods: &[]cashu.SwapMintMethod{
					bolt11Method,
				},
				Disabled:  &b,
				Supported: nil,
			}

		default:
			nuts[nut] = cashu.SwapMintInfo{
				Disabled:  &b,
				Methods:   nil,
				Supported: nil,
			}
		}
	}

	for _, nut := range optionalNuts {
		b := true
		switch nut {
		case "15":
			bolt11Method := cashu.SwapMintMethod{
				Method:    cashu.MethodBolt11,
				Unit:      cashu.Sat.String(),
				MinAmount: 0,
				MaxAmount: 0,
				Options:   nil,
				Commands:  nil,
			}

			nuts[nut] = cashu.SwapMintInfo{
				Methods: &[]cashu.SwapMintMethod{
					bolt11Method,
				},
				Disabled:  nil,
				Supported: nil,
			}
		case "17":

			wsMethod := make(map[string][]cashu.SwapMintMethod)

			bolt11Method := cashu.SwapMintMethod{
				Method:    cashu.MethodBolt11,
				Unit:      cashu.Sat.String(),
				MinAmount: 0,
				MaxAmount: 0,
				Options:   nil,
				Commands: []cashu.SubscriptionKind{
					cashu.Bolt11MeltQuote,
					cashu.Bolt11MintQuote,
					cashu.ProofStateWs,
				},
			}
			wsMethod["supported"] = []cashu.SwapMintMethod{bolt11Method}

			nuts[nut] = wsMethod

		case "20":
			wsMethod := make(map[string]bool)

			wsMethod["supported"] = true

			nuts[nut] = wsMethod

		case "21":
			formatedDiscoveryUrl := m.Config.MINT_AUTH_OICD_URL + "/.well-known/openid-configuration"
			protectedRoutes := cashu.Nut21Info{
				OpenIdDiscovery: formatedDiscoveryUrl,
				ClientId:        m.Config.MINT_AUTH_OICD_CLIENT_ID,
				ProtectedRoutes: cashu.ConvertRouteListToProtectedRouteList(m.Config.MINT_AUTH_CLEAR_AUTH_URLS),
			}

			nuts[nut] = protectedRoutes
		case "22":
			protectedRoutes := cashu.Nut22Info{
				BatMaxMint:      m.Config.MINT_AUTH_MAX_BLIND_TOKENS,
				ProtectedRoutes: cashu.ConvertRouteListToProtectedRouteList(m.Config.MINT_AUTH_BLIND_AUTH_URLS),
			}

			nuts[nut] = protectedRoutes

		default:
			nuts[nut] = cashu.SwapMintInfo{
				Supported: &b,
				Methods:   nil,
				Disabled:  nil,
			}
		}
	}

	response := cashu.GetInfoResponse{
		Name:            m.Config.NAME,
		Version:         "nutmix/" + utils.AppVersion,
		Pubkey:          m.MintPubkey,
		Description:     m.Config.DESCRIPTION,
		DescriptionLong: m.Config.DESCRIPTION_LONG,
		Motd:            m.Config.MOTD,
		Contact:         contacts,
		Nuts:            nuts,
		IconUrl:         m.Config.IconUrl,
		TosUrl:          m.Config.TosUrl,
		Time:            time.Now().Unix(),
	}

	return response
}
