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
					logger.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err).Error())
					mint.MintDB.Rollback(ctx, tx)

				} else if err != nil {
					logger.Error(fmt.Errorf("\n Rolling back  because of failure %+v\n", err).Error())
					mint.MintDB.Rollback(ctx, tx)
				} else {
					err = mint.MintDB.Commit(context.Background(), tx)
					if err != nil {
						logger.Error(fmt.Errorf("\n Failed to commit transaction: %+v \n", err).Error())
					}
				}
			}()

			swaps, err := mint.MintDB.GetLiquiditySwapsByStates([]utils.SwapState{
				utils.MintWaitingPaymentRecv,
				utils.LightningPaymentPending,
				utils.WaitingUserConfirmation,
			})

			if err != nil {
				logger.Warn(
					"mint.MintDB.GetLiquiditySwapsByStates()",
					slog.String(utils.LogExtraInfo, err.Error()))
			}

			for _, swap := range swaps {
				logger.Debug(fmt.Sprintf("Checking out swap. %v", swap.Id))

				swapTx, err := mint.MintDB.SubTx(ctx, tx)
				if err != nil {
					logger.Debug(
						"Could not get swapTx for swap",
						slog.String(utils.LogExtraInfo, err.Error()),
					)
					return
				}

				now := time.Now().Unix()

				if now > int64(swap.Expiration) {
					err := mint.MintDB.ChangeLiquiditySwapState(swapTx, swap.Id, utils.Expired)
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
					mint.MintDB.Rollback(ctx, swapTx)
					continue
				}

				payHash := hex.EncodeToString(decodedInvoice.PaymentHash[:])

				switch swap.Type {
				case utils.LiquidityIn:
					status, _, err := mint.LightningBackend.CheckReceived(payHash, decodedInvoice)
					if err != nil {
						logger.Warn(
							"mint.LightningBackend.CheckReceived(payHash)",
							slog.String(utils.LogExtraInfo, err.Error()))
						mint.MintDB.Rollback(ctx, swapTx)

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

				case utils.LiquidityOut:
					status, _, _, err := mint.LightningBackend.CheckPayed(payHash, decodedInvoice)
					if err != nil {
						logger.Warn(
							"mint.LightningBackend.CheckPayed(payHash)",
							slog.String(utils.LogExtraInfo, err.Error()))
						mint.MintDB.Rollback(ctx, swapTx)

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

				}

				err = mint.MintDB.ChangeLiquiditySwapState(swapTx, swap.Id, swap.State)
				if err != nil {
					logger.Warn(
						"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
						slog.String(utils.LogExtraInfo, err.Error()))
					mint.MintDB.Rollback(ctx, swapTx)

				}

				logger.Debug(fmt.Sprintf("Commiting swap. %v", swap.Id))
				err = mint.MintDB.Commit(context.Background(), swapTx)
				if err != nil {
					logger.Error(fmt.Errorf("\n Could not commit sub transaction: %+v \n", err).Error())
				}
			}

		}()
		time.Sleep(40 * time.Second)

	}

}
