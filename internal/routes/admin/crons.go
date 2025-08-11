package admin

import (
	"context"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

func CheckStatusOfLiquiditySwaps(mint *m.Mint) {

	for {
		func() {
			ctx := context.Background()
			tx, err := mint.MintDB.GetTx(ctx)
			if err != nil {
				slog.Debug(
					"Could not get db transactions",
					slog.String(utils.LogExtraInfo, err.Error()),
				)
				return
			}

			defer func() {
				if p := recover(); p != nil {
					slog.Error("Rolling back because of failure", slog.Any("error", err))
					mint.MintDB.Rollback(ctx, tx)

				} else if err != nil {
					slog.Error("Rolling back because of failure", slog.Any("error", err))
					mint.MintDB.Rollback(ctx, tx)
				} else {
					err = mint.MintDB.Commit(context.Background(), tx)
					if err != nil {
						slog.Error("Failed to commit transaction", slog.Any("error", err))
					}
				}
			}()

			swaps, err := mint.MintDB.GetLiquiditySwapsByStates([]utils.SwapState{
				utils.MintWaitingPaymentRecv,
				utils.LightningPaymentPending,
				utils.WaitingUserConfirmation,
			})

			if err != nil {
				slog.Warn(
					"mint.MintDB.GetLiquiditySwapsByStates()",
					slog.String(utils.LogExtraInfo, err.Error()))
			}

			for _, swap := range swaps {
				slog.Debug("Checking out swap", slog.String("swap_id", swap.Id))

				swapTx, err := mint.MintDB.SubTx(ctx, tx)
				if err != nil {
					slog.Debug(
						"Could not get swapTx for swap",
						slog.String(utils.LogExtraInfo, err.Error()),
					)
					return
				}

				now := time.Now().Unix()

				if now > int64(swap.Expiration) {
					err := mint.MintDB.ChangeLiquiditySwapState(swapTx, swap.Id, utils.Expired)
					if err != nil {
						slog.Warn(
							"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
							slog.String(utils.LogExtraInfo, err.Error()))
						continue
					}
				}
				decodedInvoice, err := zpay32.Decode(swap.LightningInvoice, mint.LightningBackend.GetNetwork())
				if err != nil {
					slog.Warn(
						"zpay32.Decode(swap.Destination, mint.LightningBackend.GetNetwork())",
						slog.String(utils.LogExtraInfo, err.Error()))
					mint.MintDB.Rollback(ctx, swapTx)
					continue
				}

				payHash := hex.EncodeToString(decodedInvoice.PaymentHash[:])

				switch swap.Type {
				case utils.LiquidityIn:
					status, _, err := mint.LightningBackend.CheckReceived(cashu.MintRequestDB{Quote: payHash}, decodedInvoice)
					if err != nil {
						slog.Warn(
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
					status, _, _, err := mint.LightningBackend.CheckPayed(payHash, decodedInvoice, swap.CheckingId)
					if err != nil {
						slog.Warn(
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
					slog.Warn(
						"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
						slog.String(utils.LogExtraInfo, err.Error()))
					mint.MintDB.Rollback(ctx, swapTx)

				}

				slog.Debug("Commiting swap", slog.String("swap_id", swap.Id))
				err = mint.MintDB.Commit(context.Background(), swapTx)
				if err != nil {
					slog.Error("Could not commit sub transaction", slog.Any("error", err))
				}
			}

		}()
		time.Sleep(40 * time.Second)

	}

}
