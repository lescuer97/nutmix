package routes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func v1MintRoutes(r *gin.Engine, mint *m.Mint, logger *slog.Logger) {
	v1 := r.Group("/v1")

	v1.GET("/keys", func(c *gin.Context) {

		keys, err := mint.Signer.GetActiveKeys()
		if err != nil {
			logger.Error(fmt.Sprintf("mint.Signer.GetActiveKeys() %+v ", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keys)

	})

	v1.GET("/keys/:id", func(c *gin.Context) {

		id := c.Param("id")

		keysets, err := mint.Signer.GetKeysById(id)

		if err != nil {
			logger.Error(fmt.Sprintf("mint.Signer.GetKeysById(id) %+v", err))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		c.JSON(200, keysets)

	})
	v1.GET("/keysets", func(c *gin.Context) {

		keys, err := mint.Signer.GetKeys()
		if err != nil {
			logger.Error(fmt.Errorf("mint.Signer.GetKeys() %w", err).Error())
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
		var baseNuts []string = []string{"1", "2", "3", "4", "5", "6"}

		var optionalNuts []string = []string{"7", "8", "9", "10", "11", "12", "17"}

		if mint.LightningBackend.ActiveMPP() {
			optionalNuts = append(optionalNuts, "15")
		}
		if mint.Config.MINT_REQUIRE_AUTH {
			optionalNuts = append(optionalNuts, "21")
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
					Method:    cashu.MethodBolt11,
					Mpp:       true,
					Unit:      cashu.Sat.String(),
					MinAmount: 0,
				}

				info := []cashu.SwapMintMethod{
					bolt11Method,
				}

				nuts[nut] = info
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

			case "21":
				protectedRoutes := cashu.Nut21Info{
					OpenIdDiscovery: mint.Config.MINT_AUTH_OICD_DISCOVERY_URL,
					ClientId:        mint.Config.MINT_AUTH_OICD_CLIENT_ID,
					ProtectedRoutes: []cashu.ProtectedRoute{
						{Method: "POST",
							Path: "/v1/auth/blind/mint",
						},
					},
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
			Version:         "NutMix/0.2.0",
			Pubkey:          mint.MintPubkey,
			Description:     mint.Config.DESCRIPTION,
			DescriptionLong: mint.Config.DESCRIPTION_LONG,
			Motd:            mint.Config.MOTD,
			Contact:         contacts,
			Nuts:            nuts,
		}

		c.JSON(200, response)
	})

	v1.POST("/swap", func(c *gin.Context) {
		var swapRequest cashu.PostSwapRequest

		err := c.BindJSON(&swapRequest)
		if err != nil {
			logger.Info("Incorrect body: %+v", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, "Malformed body request")
			return
		}

		var AmountSignature uint64

		if len(swapRequest.Inputs) == 0 || len(swapRequest.Outputs) == 0 {
			logger.Info("Inputs or Outputs are empty")
			c.JSON(400, "Inputs or Outputs are empty")
			return
		}

		AmountProofs, SecretsList, err := utils.GetAndCalculateProofsValues(&swapRequest.Inputs)
		if err != nil {
			logger.Warn("utils.GetProofsValues(&swapRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, "Problem processing proofs")
			return
		}

		for _, output := range swapRequest.Outputs {
			AmountSignature += output.Amount
		}
		keysets, err := mint.Signer.GetKeys()
		if err != nil {
			logger.Warn("mint.Signer.GetKeys()", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNKNOWN, nil))
			return
		}

		_, err = mint.CheckProofsAreSameUnit(swapRequest.Inputs, keysets.Keysets)

		if err != nil {
			logger.Warn("CheckProofsAreSameUnit", slog.String(utils.LogExtraInfo, err.Error()))
			detail := "Proofs are not the same unit"
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNIT_NOT_SUPPORTED, &detail))
			return
		}

		// check for needed amount of fees
		fee, err := cashu.Fees(swapRequest.Inputs, keysets.Keysets)
		if err != nil {
			logger.Warn("cashu.Fees(swapRequest.Inputs, mint.Keysets[unit.String()])", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.KEYSET_NOT_KNOW, nil))
			return
		}

		if AmountProofs < (uint64(fee) + AmountSignature) {
			logger.Info(fmt.Sprintf("didn't provide enough fees. ProofAmount: %v, needed Proofs: %v", AmountProofs, (uint64(fee) + AmountSignature)))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.TRANSACTION_NOT_BALANCED, nil))
			return
		}

		ctx := context.Background()
		tx, err := mint.MintDB.GetTx(ctx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.GetTx(ctx): %w", err))
			return
		}
		defer mint.MintDB.Rollback(ctx, tx)

		// check if we know any of the proofs
		knownProofs, err := mint.MintDB.GetProofsFromSecretCurve(tx, SecretsList)

		if err != nil {
			logger.Error("mint.MintDB.GetProofsFromSecretCurve(tx, SecretsList)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.UNKNOWN, nil))
			return
		}

		if len(knownProofs) != 0 {
			logger.Warn("Proofs already spent", slog.String(utils.LogExtraInfo, fmt.Sprintf("know proofs: %+v", knownProofs)))
			c.JSON(400, cashu.ErrorCodeToResponse(cashu.TOKEN_ALREADY_SPENT, nil))
			return
		}

		swapRequest.Inputs.SetProofsState(cashu.PROOF_PENDING)

		// send proofs to database
		err = mint.MintDB.SaveProof(tx, swapRequest.Inputs)

		if err != nil {
			logger.Error("mint.MintDB.SaveProof(tx, swapRequest.Inputs)", slog.String(utils.LogExtraInfo, err.Error()))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(403, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		err = mint.Signer.VerifyProofs(swapRequest.Inputs, swapRequest.Outputs)

		if err != nil {
			logger.Warn(" mint.Signer.VerifyProofs(swapRequest.Inputs, swapRequest.Outputs, unit)", slog.String(utils.LogExtraInfo, err.Error()))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(403, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		// sign the outputs
		blindedSignatures, recoverySigsDb, err := mint.Signer.SignBlindMessages(swapRequest.Outputs)

		if err != nil {
			logger.Error("mint.Signer.SignBlindMessages(swapRequest.Outputs, cashu.Sat)", slog.String(utils.LogExtraInfo, err.Error()))
			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(400, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		response := cashu.PostSwapResponse{
			Signatures: blindedSignatures,
		}

		err = mint.MintDB.SetProofsState(tx, swapRequest.Inputs, cashu.PROOF_SPENT)
		if err != nil {
			logger.Warn("mint.MintDB.SetProofsState(tx,swapRequest.Inputs , cashu.PROOF_SPENT)", slog.String(utils.LogExtraInfo, err.Error()))

			errorCode, details := utils.ParseErrorToCashuErrorCode(err)
			c.JSON(403, cashu.ErrorCodeToResponse(errorCode, details))
			return
		}

		err = mint.MintDB.SaveRestoreSigs(tx, recoverySigsDb)
		if err != nil {
			logger.Error("database.SetRestoreSigs", slog.String(utils.LogExtraInfo, err.Error()))
			logger.Error("recoverySigsDb", slog.String(utils.LogExtraInfo, fmt.Sprintf("%+v", recoverySigsDb)))
			c.JSON(200, response)
			return
		}
		err = mint.MintDB.Commit(ctx, tx)
		if err != nil {
			c.Error(fmt.Errorf("mint.MintDB.Commit(ctx tx). %w", err))
			return
		}

		c.JSON(200, response)
	})

	v1.POST("/checkstate", func(c *gin.Context) {
		var checkStateRequest cashu.PostCheckStateRequest
		err := c.BindJSON(&checkStateRequest)
		if err != nil {
			logger.Info("c.BindJSON(&checkStateRequest)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, "Malformed Body")
			return
		}

		checkStateResponse := cashu.PostCheckStateResponse{
			States: make([]cashu.CheckState, 0),
		}

		states, err := m.CheckProofState(mint, checkStateRequest.Ys)
		checkStateResponse.States = states

		c.JSON(200, checkStateResponse)

	})
	v1.POST("/restore", func(c *gin.Context) {
		var restoreRequest cashu.PostRestoreRequest
		err := c.BindJSON(&restoreRequest)

		if err != nil {
			logger.Info("c.BindJSON(&restoreRequest)", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(400, "Malformed body request")
			return
		}

		blindingFactors := []string{}

		for _, output := range restoreRequest.Outputs {
			blindingFactors = append(blindingFactors, output.B_)
		}

		blindRecoverySigs, err := mint.MintDB.GetRestoreSigsFromBlindedMessages(blindingFactors)
		if err != nil {
			logger.Error("database.GetRestoreSigsFromBlindedMessages", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Opps!, something went wrong")
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
