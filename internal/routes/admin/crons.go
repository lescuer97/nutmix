package admin

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
	"log/slog"
	"time"
)

func CheckStatusOfLiquiditySwaps(mint *m.Mint, logger *slog.Logger) {

	for {
		func() {
			ctx := context.Background()
			tx, err := mint.MintDB.GetTx(ctx)
			if err != nil {
				logger.Debug(
					"Could not get db transactions",
					slog.String(utils.LogExtraInfo, err.Error()),
				)
				return
			}

			defer func() {
				if p := recover(); p != nil {
					logger.Error("\n Rolling back  because of failure %+v\n", p)
					tx.Rollback(ctx)
				} else if err != nil {
					logger.Error(fmt.Sprintf("\n Rolling back  because of failure %+v\n", err))
					tx.Rollback(ctx)
				} else {
					err = tx.Commit(ctx)
					if err != nil {
						logger.Error(fmt.Sprintf("\n Failed to commit transaction: %+v \n", err))
					}
				}
			}()

			swaps, err := mint.MintDB.GetLiquiditySwapsByStates([]utils.SwapState{
				utils.MintWaitingPaymentRecv,
				utils.LightningPaymentFail,
				utils.LightningPaymentPending,
				utils.UnknownProblem,
			})

			if err != nil {
				logger.Warn(
					"mint.MintDB.GetLiquiditySwapsByStates()",
					slog.String(utils.LogExtraInfo, err.Error()))
			}

			for _, swap := range swaps {
				now := time.Now().Unix()

				if now > int64(swap.Expiration) {
					err := mint.MintDB.ChangeLiquiditySwapState(tx, swap.Id, utils.Expired)
					if err != nil {
						logger.Warn(
							"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
							slog.String(utils.LogExtraInfo, err.Error()))
						continue
					}
				}

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
				case lightning.PENDING:
					swap.State = utils.LightningPaymentPending
				case lightning.FAILED:
					swap.State = utils.LightningPaymentFail
				}

				err = mint.MintDB.ChangeLiquiditySwapState(tx, swap.Id, swap.State)
				if err != nil {
					logger.Warn(
						"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
						slog.String(utils.LogExtraInfo, err.Error()))
					continue

				}

			}

		}()
		time.Sleep(40 * time.Second)

	}

}
