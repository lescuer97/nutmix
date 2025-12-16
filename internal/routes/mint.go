package routes

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func v1MintRoutes(r *gin.Engine, mint *m.Mint) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {

		keys, err := mint.Signer.GetActiveKeys()
		if err != nil {
			slog.Error("mint.Signer.GetActiveKeys()", slog.Any("error", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keys)

	})

	v1.GET("/keys/:id", func(c *gin.Context) {

		id := c.Param("id")

		keysets, err := mint.Signer.GetKeysById(id)

		if err != nil {
			slog.Error("mint.Signer.GetKeysById(id)", slog.Any("error", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keysets)

	})
	v1.GET("/keysets", func(c *gin.Context) {

		keys, err := mint.Signer.GetKeysets()
		if err != nil {
			slog.Error("mint.Signer.GetKeys()", slog.Any("error", err))
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)
	})

	v1.GET("/info", func(c *gin.Context) {

		contacts := []cashu.ContactInfo{}

		email := mint.Config.EMAIL

		if len(email) > 0 {
			contacts = append(contacts, cashu.ContactInfo{
				Method: "email",
				Info:   email,
			})
		}

		nostr := mint.Config.NOSTR

		if len(nostr) > 0 {
			contacts = append(contacts, cashu.ContactInfo{
				Method: "nostr",
				Info:   nostr,
			})
		}

		nuts := make(map[string]any)
		var baseNuts = []string{"1", "2", "3", "4", "5", "6"}

		var optionalNuts = []string{"7", "8", "9", "10", "11", "12", "17", "20"}

		if mint.LightningBackend.ActiveMPP() {
			optionalNuts = append(optionalNuts, "15")
		}
		if mint.Config.MINT_REQUIRE_AUTH {
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
				}

				if mint.Config.PEG_IN_LIMIT_SATS != nil {
					bolt11Method.MaxAmount = *mint.Config.PEG_IN_LIMIT_SATS
				}

				descriptionEnabled := mint.LightningBackend.DescriptionSupport()
				bolt11Method.Options = &cashu.SwapMintMethodOptions{
					Description: &descriptionEnabled,
				}

				nuts[nut] = cashu.SwapMintInfo{
					Methods: &[]cashu.SwapMintMethod{
						bolt11Method,
					},
					Disabled: &b,
				}
				if entry, ok := nuts[nut]; ok {

					mintInfo := entry.(cashu.SwapMintInfo)
					// Then we modify the copy
					mintInfo.Disabled = &mint.Config.PEG_OUT_ONLY

					// Then we reassign map entry
					nuts[nut] = mintInfo
				}

			case "5":
				bolt11Method := cashu.SwapMintMethod{
					Method:    cashu.MethodBolt11,
					Unit:      cashu.Sat.String(),
					MinAmount: 0,
				}

				if mint.Config.PEG_OUT_LIMIT_SATS != nil {
					bolt11Method.MaxAmount = *mint.Config.PEG_OUT_LIMIT_SATS
				}

				nuts[nut] = cashu.SwapMintInfo{
					Methods: &[]cashu.SwapMintMethod{
						bolt11Method,
					},
					Disabled: &b,
				}

			default:
				nuts[nut] = cashu.SwapMintInfo{
					Disabled: &b,
				}

			}
		}

		for _, nut := range optionalNuts {
			b := true
			switch nut {
			case "15":
				bolt11Method := cashu.SwapMintMethod{
					Method: cashu.MethodBolt11,
					Unit:   cashu.Sat.String(),
				}

				nuts[nut] = cashu.SwapMintInfo{
					Methods: &[]cashu.SwapMintMethod{
						bolt11Method,
					},
				}
			case "17":

				wsMethod := make(map[string][]cashu.SwapMintMethod)

				bolt11Method := cashu.SwapMintMethod{
					Method: cashu.MethodBolt11,
					Unit:   cashu.Sat.String(),
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
				formatedDiscoveryUrl := mint.Config.MINT_AUTH_OICD_URL + "/.well-known/openid-configuration"
				protectedRoutes := cashu.Nut21Info{
					OpenIdDiscovery: formatedDiscoveryUrl,
					ClientId:        mint.Config.MINT_AUTH_OICD_CLIENT_ID,
					ProtectedRoutes: cashu.ConvertRouteListToProtectedRouteList(mint.Config.MINT_AUTH_CLEAR_AUTH_URLS),
				}

				nuts[nut] = protectedRoutes
			case "22":
				protectedRoutes := cashu.Nut22Info{
					BatMaxMint:      mint.Config.MINT_AUTH_MAX_BLIND_TOKENS,
					ProtectedRoutes: cashu.ConvertRouteListToProtectedRouteList(mint.Config.MINT_AUTH_BLIND_AUTH_URLS),
				}

				nuts[nut] = protectedRoutes

			default:
				nuts[nut] = cashu.SwapMintInfo{
					Supported: &b,
				}

			}
		}

		response := cashu.GetInfoResponse{
			Name:            mint.Config.NAME,
			Version:         "nutmix/" + utils.AppVersion,
			Pubkey:          mint.MintPubkey,
			Description:     mint.Config.DESCRIPTION,
			DescriptionLong: mint.Config.DESCRIPTION_LONG,
			Motd:            mint.Config.MOTD,
			Contact:         contacts,
			Nuts:            nuts,
			IconUrl:         mint.Config.IconUrl,
			TosUrl:          mint.Config.TosUrl,
			Time:            time.Now().Unix(),
		}

		c.JSON(200, response)
	})

	v1.POST("/swap", func(c *gin.Context) {
		var swapRequest cashu.PostSwapRequest

		err := c.BindJSON(&swapRequest)
		if err != nil {
			slog.Info("Incorrect body", slog.Any("error", err))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		if len(swapRequest.Inputs) == 0 || len(swapRequest.Outputs) == 0 {
			slog.Info("Inputs or Outputs are empty")
			c.JSON(400, "Inputs or Outputs are empty")
			return
		}

		_, SecretsList, err := utils.GetAndCalculateProofsValues(&swapRequest.Inputs)
		if err != nil {
			slog.Warn("utils.GetAndCalculateProofsValues(&swapRequest.Inputs)", slog.Any("error", err))
			c.JSON(400, "Problem processing proofs")
			return
		}

		// Verify spending conditions - EXCLUSIVE paths following CDK pattern
		hasSigAll, err := cashu.ProofsHaveSigAll(swapRequest.Inputs)
		if err != nil {
			slog.Error(fmt.Errorf("cashu.ProofsHaveSigAll(swapRequest.Inputs). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		if hasSigAll {
			// SIG_ALL path: verify all conditions match and signature is valid against combined message
			err = swapRequest.ValidateSigflag()
			if err != nil {
				slog.Error(fmt.Errorf("swapRequest.ValidateSigflag(). %w", err).Error())
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}
		} else {
			// Individual verification path: verify each proof's P2PK/HTLC spend conditions
			err = mint.VerifyProofsSpendConditions(swapRequest.Inputs)
			if err != nil {
				slog.Error(fmt.Errorf("mint.VerifyProofsSpendConditions(swapRequest.Inputs). %w", err).Error())
				errorCode, details := utils.ParseErrorToCashuErrorCode(err)
				c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
				return
			}
		}

		// Always verify BDHKE cryptographic signatures (regardless of SIG_ALL)
		err = mint.VerifyProofsBDHKE(swapRequest.Inputs)
		if err != nil {
			slog.Error(fmt.Errorf("mint.VerifyProofsBDHKE(swapRequest.Inputs). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		ctx := context.Background()
		preparationTx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err))
			return
		}
		defer func() {
			if err := mint.MintDB.Rollback(ctx, preparationTx); err != nil {
				slog.Warn("rollback error", slog.Any("error", err))
			}
		}()

		// VerifyInputsAndOutputs now only checks balance/unit and BDHKE (spend conditions already verified)
		err = mint.VerifyInputsAndOutputs(preparationTx, swapRequest.Inputs, swapRequest.Outputs)
		if err != nil {
			slog.Error(fmt.Errorf("mint.VerifyInputsAndOutputs(swapRequest.Inputs, swapRequest.Outputs). %w", err).Error())
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		// check if we know any of the proofs
		knownProofs, err := mint.MintDB.GetProofsFromSecretCurve(preparationTx, SecretsList)

		if err != nil {
			slog.Error("mint.MintDB.GetProofsFromSecretCurve(tx, SecretsList)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNKNOWN, nil))
			return
		}

		if len(knownProofs) != 0 {
			slog.Debug("Proofs already spent", slog.Any("known_proofs", knownProofs))
			for _, p := range knownProofs {
				if p.State == cashu.PROOF_PENDING {
					c.JSON(400, cashu.ErrorCodeToResponse(cashu.PROOFS_PENDING, nil))
					return
				}
			}

			c.JSON(400, cashu.ErrorCodeToResponse(cashu.PROOF_ALREADY_SPENT, nil))
			return
		}

		swapRequest.Inputs.SetProofsState(cashu.PROOF_PENDING)

		// send proofs to database
		err = mint.MintDB.SaveProof(preparationTx, swapRequest.Inputs)

		if err != nil {
			slog.Error("mint.MintDB.SaveProof(tx, swapRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(403, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		err = mint.MintDB.Commit(ctx, preparationTx)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		// sign the outputs
		blindedSignatures, recoverySigsDb, err := mint.Signer.SignBlindMessages(swapRequest.Outputs)
		if err != nil {
			slog.Error("mint.Signer.SignBlindMessages(swapRequest.Outputs)", slog.String(utils.LogExtraInfo, err.Error()))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		response := cashu.PostSwapResponse{
			Signatures: blindedSignatures,
		}
		afterSigningTx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err))
			return
		}
		defer func() {
			if err := mint.MintDB.Rollback(ctx, afterSigningTx); err != nil {
				slog.Warn("rollback error", slog.Any("error", err))
			}
		}()

		swapRequest.Inputs.SetProofsState(cashu.PROOF_SPENT)
		err = mint.MintDB.SetProofsState(afterSigningTx, swapRequest.Inputs, cashu.PROOF_SPENT)
		if err != nil {
			slog.Warn("mint.MintDB.SetProofsState(tx,swapRequest.Inputs , cashu.PROOF_SPENT)", slog.Any("error", err))

			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(403, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		err = mint.MintDB.SaveRestoreSigs(afterSigningTx, recoverySigsDb)
		if err != nil {
			slog.Error("database.SetRestoreSigs", slog.String(utils.LogExtraInfo, err.Error()))
			slog.Error("recoverySigsDb", slog.Any("recovery_sigs", recoverySigsDb))
			c.JSON(200, response)
			return
		}
		err = mint.MintDB.Commit(ctx, afterSigningTx)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.Commit(ctx, afterSigningTx). %w", err))
			return
		}

		go mint.Observer.SendProofsEvent(swapRequest.Inputs)
		c.JSON(200, response)
	})

	v1.POST("/checkstate", func(c *gin.Context) {
		var checkStateRequest cashu.PostCheckStateRequest
		err := c.BindJSON(&checkStateRequest)
		if err != nil {
			slog.Info("c.BindJSON(&checkStateRequest)", slog.Any("error", err))
			c.JSON(400, "Malformed Body")
			return
		}

		checkStateResponse := cashu.PostCheckStateResponse{
			States: make([]cashu.CheckState, 0),
		}

		states, err := m.CheckProofState(mint, checkStateRequest.Ys)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		checkStateResponse.States = states

		c.JSON(200, checkStateResponse)

	})
	v1.POST("/restore", func(c *gin.Context) {
		var restoreRequest cashu.PostRestoreRequest
		err := c.BindJSON(&restoreRequest)

		if err != nil {
			slog.Info("c.BindJSON(&restoreRequest)", slog.Any("error", err))
			c.JSON(400, "Malformed body request")
			return
		}

		blindingFactors := []string{}

		for _, output := range restoreRequest.Outputs {
			blindingFactors = append(blindingFactors, output.B_.String())
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err))
			return
		}
		blindRecoverySigs, err := mint.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors)
		if err != nil {
			slog.Error("mint.MintDB.GetRestoreSigsFromBlindedMessages(tx, blindingFactors)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Opps!, something went wrong")
			return
		}
		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			_ = c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		restoredBlindSigs := []cashu.BlindSignature{}
		restoredBlindMessage := []cashu.BlindedMessage{}

		for _, sigRecover := range blindRecoverySigs {
			restoredSig, restoredMessage := sigRecover.GetSigAndMessage()
			restoredBlindSigs = append(restoredBlindSigs, restoredSig)
			restoredBlindMessage = append(restoredBlindMessage, restoredMessage)
		}

		c.JSON(200, cashu.PostRestoreResponse{
			Outputs:    restoredBlindMessage,
			Signatures: restoredBlindSigs,
			Promises:   restoredBlindSigs,
		})
	})

}
