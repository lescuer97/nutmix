package admin

import (
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func CheckStatusOfLiquiditySwaps(mint *m.Mint, logger *slog.Logger) {

	for {

		swaps, err := mint.MintDB.GetLiquiditySwapsByStates([]utils.SwapState{utils.WaitingBoltzTXConfirmations,
			utils.BoltzWaitingPayment,
			utils.MintWaitingPaymentRecv,
			utils.LightnigPaymentFail,
			utils.UnknownProblem,
		})
		if err != nil {
			logger.Warn(
				"mint.MintDB.GetLiquiditySwapsByStates()",
				slog.String(utils.LogExtraInfo, err.Error()))
		}
		defer func() {
			if p := recover(); p != nil {
				logger.Warn(
					"Something paniqued",
					slog.String(utils.LogExtraInfo, err.Error()))
			} else if err != nil {
				logger.Warn(
					"Some error happened",
					slog.String(utils.LogExtraInfo, err.Error()))
			}
		}()

		for _, swap := range swaps {
			now := time.Now().Unix()

			if now > int64(swap.Expiration) {
				err := mint.MintDB.ChangeLiquiditySwapState(swap.Id, utils.Expired)
				if err != nil {
					logger.Warn(
						"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
						slog.String(utils.LogExtraInfo, err.Error()))
					continue
				}
			}

			switch swap.Type {
			case utils.LiquidityIn:
				decodedInvoice, err := zpay32.Decode(swap.LightningInvoice, mint.LightningBackend.GetNetwork())
				if err != nil {
					logger.Warn(
						"zpay32.Decode(swap.Destination, mint.LightningBackend.GetNetwork())",
						slog.String(utils.LogExtraInfo, err.Error()))
					continue
				}

				payHash := hex.EncodeToString(decodedInvoice.PaymentHash[:])
				status, _, err := mint.LightningBackend.CheckPayed(payHash)
				if err != nil {
					logger.Warn(
						"mint.LightningBackend.CheckPayed(payHash)",
						slog.String(utils.LogExtraInfo, err.Error()))
					continue
				}

				switch status {
				case lightning.SETTLED:
					swap.State = utils.Finished
				case lightning.FAILED:
					swap.State = utils.LightnigPaymentFail
				}

				err = mint.MintDB.ChangeLiquiditySwapState(swap.Id, swap.State)
				if err != nil {
					logger.Warn(
						"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
						slog.String(utils.LogExtraInfo, err.Error()))
					continue
				}

			}

		}

		time.Sleep(10 * time.Second)
	}

}
