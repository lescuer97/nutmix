package admin

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"github.com/lightningnetwork/lnd/zpay32"
)

// newLiquidity is a channel that will be used to send new liquidity swap ids to the checked
func CheckStatusOfLiquiditySwaps(mint *m.Mint, newLiquidity chan string) {
	ctx := context.Background()
	tx, err := mint.MintDB.GetTx(ctx)
	if err != nil {
		slog.Warn(
			"Could not get db transactions",
			slog.String(utils.LogExtraInfo, err.Error()),
		)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			slog.Warn("Rolling back because of failure", slog.Any("error", err))
			if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
				slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
			}

		} else if err != nil {
			slog.Warn("Rolling back because of failure", slog.Any("error", err))
			if rollbackErr := mint.MintDB.Rollback(ctx, tx); rollbackErr != nil {
				slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
			}
		}
	}()
	swaps, err := mint.MintDB.GetLiquiditySwapsByStates(tx, []utils.SwapState{
		utils.MintWaitingPaymentRecv,
		utils.LightningPaymentPending,
	})

	if err != nil {
		slog.Warn(
			"mint.MintDB.GetLiquiditySwapsByStates()",
			slog.Any("error", err.Error()))
	}
	err = tx.Commit(ctx)
	if err != nil {
		slog.Error("Could not commit transaction", slog.Any("error", err))
	}

	for {
		func() {

			// check if there are new liquidity swaps to check
			select {
			case swapId := <-newLiquidity:
				swaps = append(swaps, swapId)
			default:
				break
			}

			for _, swapId := range swaps {
				func() {
					slog.Debug("Checking out swap", slog.String("swap_id", swapId))

					swapTx, err := mint.MintDB.GetTx(ctx)
					if err != nil {
						slog.Warn(
							"Could not get db transactions",
							slog.String(utils.LogExtraInfo, err.Error()),
						)
						return
					}
					defer func() {
						if p := recover(); p != nil {
							slog.Warn("Rolling back because of failure", slog.Any("error", err))
							if rollbackErr := mint.MintDB.Rollback(ctx, swapTx); rollbackErr != nil {
								slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
							}

						} else if err != nil {
							slog.Warn("Rolling back because of failure", slog.Any("error", err))
							if rollbackErr := mint.MintDB.Rollback(ctx, swapTx); rollbackErr != nil {
								slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
							}
						}
					}()

					swap, err := mint.MintDB.GetLiquiditySwapById(swapTx, swapId)
					if err != nil {
						slog.Warn(
							"Could not get swap",
							slog.String(utils.LogExtraInfo, err.Error()),
						)
						return
					}
					err = swapTx.Commit(ctx)
					if err != nil {
						slog.Error("Could not commit sub transaction", slog.Any("error", err))
						return
					}

					decodedInvoice, err := zpay32.Decode(swap.LightningInvoice, mint.LightningBackend.GetNetwork())
					if err != nil {
						slog.Warn(
							"zpay32.Decode(swap.Destination, mint.LightningBackend.GetNetwork())",
							slog.String(utils.LogExtraInfo, err.Error()))
						return
					}

					payHash := hex.EncodeToString(decodedInvoice.PaymentHash[:])

					switch swap.Type {
					case utils.LiquidityIn:
						slog.Debug("Checking in swap", slog.String("swap_id", swap.Id))
						status, _, err := mint.LightningBackend.CheckReceived(cashu.MintRequestDB{Quote: payHash}, decodedInvoice)
						if err != nil {
							slog.Warn(
								"mint.LightningBackend.CheckReceived(payHash)",
								slog.String(utils.LogExtraInfo, err.Error()))

							return
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
						slog.Debug("Checking out swap", slog.String("swap_id", swap.Id))
						status, _, _, err := mint.LightningBackend.CheckPayed(payHash, decodedInvoice, swap.CheckingId)
						if err != nil {
							slog.Warn(
								"mint.LightningBackend.CheckPayed(payHash)",
								slog.Any("error", err),
								slog.String("swap_id", swap.Id),
								slog.String("invoice", swap.LightningInvoice),
							)

							return
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

					afterCheckTx, err := mint.MintDB.GetTx(ctx)
					if err != nil {
						slog.Warn(
							"Could not get db transactions",
							slog.String(utils.LogExtraInfo, err.Error()),
						)
						return
					}
					defer func() {
						if p := recover(); p != nil {
							slog.Warn("Rolling back because of failure", slog.Any("error", err))
							if rollbackErr := mint.MintDB.Rollback(ctx, afterCheckTx); rollbackErr != nil {
								slog.Error("Failed to rollback transaction", slog.Any("error", rollbackErr))
							}
						}
					}()
					err = mint.MintDB.ChangeLiquiditySwapState(afterCheckTx, swap.Id, swap.State)
					if err != nil {
						slog.Warn(
							"mint.MintDB.ChangeLiquiditySwapState(swap.Id,utils.Expired)",
							slog.String(utils.LogExtraInfo, err.Error()))

						return
					}

					slog.Debug("Committing swap", slog.String("swap_id", swap.Id))
					err = afterCheckTx.Commit(ctx)
					if err != nil {
						slog.Error("Could not commit sub transaction", slog.Any("error", err))
						return
					}

					if swap.State == utils.Finished || swap.State == utils.Expired || swap.State == utils.LightningPaymentFail {
						swaps = slices.DeleteFunc(swaps, func(id string) bool {
							return id == swap.Id
						})
					}
				}()
			}

		}()
		slog.Debug("Sleeping for 2 seconds", slog.String("swaps", fmt.Sprintf("%v", swaps)))
		time.Sleep(2 * time.Second)

	}

}
