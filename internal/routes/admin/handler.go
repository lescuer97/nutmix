package admin

import (
	"context"
	"fmt"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

type adminHandler struct {
	mint *mint.Mint
}

func newAdminHandler(mint *mint.Mint) adminHandler {
	if mint == nil {
		log.Panicf("mint should never be null in the adminHandler")
	}
	return adminHandler{
		mint: mint,
	}
}

func (a *adminHandler) getKeysets(filterUnits []string) (map[string][]templates.KeysetData, []string, error) {
	keysets, err := a.mint.Signer.GetKeysets()
	if err != nil {
		return nil, nil, fmt.Errorf("a.mint.Signer.GetKeysets(). %w", err)
	}
	authKeysets, err := a.mint.Signer.GetAuthKeys()
	if err != nil {
		return nil, nil, fmt.Errorf("a.mint.Signer.GetAuthKeys(). %w", err)
	}

	keysetMap := make(map[string][]templates.KeysetData)
	for _, seed := range keysets.Keysets {
		if len(filterUnits) > 0 {
			if !slices.Contains(filterUnits, seed.Unit) {
				continue
			}
		}

		var expireTime *time.Time = nil
		if seed.FinalExpiry != nil {
			unixTime := time.Unix(int64(*seed.FinalExpiry), 0)
			expireTime = &unixTime
		}

		keysetMap[seed.Unit] = append(keysetMap[seed.Unit], templates.KeysetData{
			Id:          seed.Id,
			Active:      seed.Active,
			Unit:        seed.Unit,
			Fees:        seed.InputFeePpk,
			Version:     seed.Version,
			CreatedAt:   0,
			ExpireLimit: expireTime,
		})
	}
	for _, seed := range authKeysets.Keysets {
		if len(filterUnits) > 0 {
			if !slices.Contains(filterUnits, seed.Unit) {
				continue
			}
		}

		var expireTime *time.Time = nil
		if seed.FinalExpiry != nil {
			unixTime := time.Unix(int64(*seed.FinalExpiry), 0)
			expireTime = &unixTime
		}

		keysetMap[seed.Unit] = append(keysetMap[seed.Unit], templates.KeysetData{
			Id:          seed.Id,
			Active:      seed.Active,
			Unit:        seed.Unit,
			Fees:        seed.InputFeePpk,
			Version:     seed.Version,
			CreatedAt:   0,
			ExpireLimit: expireTime,
		})
	}

	// order the keysets by version
	for unit, ranges := range keysetMap {
		sort.Slice(ranges, func(i, j int) bool { return ranges[i].Version > ranges[j].Version })
		keysetMap[unit] = ranges
	}

	// create ordered list of units
	orderedUnits := make([]string, 0, len(keysetMap))
	for unit := range keysetMap {
		orderedUnits = append(orderedUnits, unit)
	}
	sort.Slice(orderedUnits, func(i, j int) bool {
		if orderedUnits[i] == "sat" {
			return true
		}
		if orderedUnits[j] == "sat" {
			return false
		}
		return orderedUnits[i] < orderedUnits[j]
	})

	return keysetMap, orderedUnits, nil
}

func (a *adminHandler) rotateKeyset(unit cashu.Unit, fee uint, expiry_hours uint) error {
	return a.mint.Signer.RotateKeyset(unit, fee, expiry_hours)
}

func (a *adminHandler) lnSatsBalance() (uint64, error) {
	balanceAmount, err := a.mint.LightningBackend.WalletBalance()
	if err != nil {
		return 0, fmt.Errorf("a.mint.LightningBackend.WalletBalance(). %w", err)
	}
	// Convert to Sat for display
	convertErr := balanceAmount.To(cashu.Sat)
	if convertErr != nil {
		return 0, fmt.Errorf("balanceAmount.To(cashu.Sat). %w", convertErr)
	}
	return balanceAmount.Amount, nil
}

func (a *adminHandler) EcashBalance(since time.Time) (templates.Balance, error) {
	statsRows, err := a.mint.MintDB.GetStatsSnapshotsBySince(context.Background(), since.Unix())
	if err != nil {
		return templates.Balance{}, fmt.Errorf("a.mint.MintDB.GetStatsSnapshotsBySince(context.Background(), since.Unix()). %w", err)
	}
	return balanceFromStatsSnapshots(statsRows), nil
}

func balanceFromStatsSnapshots(rows []database.StatsSnapshot) templates.Balance {
	proofsTotalAmountValue := uint64(0)
	proofsTotalQuantityValue := uint64(0)
	blindSigsTotalAmountValue := uint64(0)
	blindSigsTotalQuantityValue := uint64(0)
	for _, row := range rows {
		for _, item := range row.ProofsSummary {
			proofsTotalAmountValue += item.Amount
			proofsTotalQuantityValue += item.Quantity
		}
		for _, item := range row.BlindSigsSummary {
			blindSigsTotalAmountValue += item.Amount
			blindSigsTotalQuantityValue += item.Quantity
		}
	}
	neededBalance := uint64(0)
	if blindSigsTotalAmountValue > proofsTotalAmountValue {
		neededBalance = blindSigsTotalAmountValue - proofsTotalAmountValue
	}
	ratioProofSigAmountSats := 0.0
	if blindSigsTotalAmountValue > 0 {
		ratioProofSigAmountSats = (float64(proofsTotalAmountValue) / float64(blindSigsTotalAmountValue)) * 100
	}
	return templates.Balance{
		ProofsAmount:      proofsTotalAmountValue,
		ProofsQuantity:    proofsTotalQuantityValue,
		BlindSigsAmount:   blindSigsTotalAmountValue,
		BlindSigsQuantity: blindSigsTotalQuantityValue,
		NeededBalance:     neededBalance,
		Ratio:             ratioProofSigAmountSats,
	}
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getTotalBalanceBlindSignaturesByTime(until time.Time) (uint64, error) {
	panic("still not implemented")
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getMintRequestsByTimeAndId(since time.Time, id *string) (uint64, error) {
	panic("still not implemented")
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getMeltRequestsByTimeAndId(since time.Time, id *string) (uint64, error) {
	panic("still not implemented")
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getLogs(until time.Time) (uint64, error) {
	panic("still not implemented")
}
