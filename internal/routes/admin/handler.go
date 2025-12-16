package admin

import (
	"fmt"
	"log"
	"slices"
	"sort"
	"time"

	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"golang.org/x/sync/errgroup"
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
		keysetMap[seed.Unit] = append(keysetMap[seed.Unit], templates.KeysetData{
			Id:      seed.Id,
			Active:  seed.Active,
			Unit:    seed.Unit,
			Fees:    seed.InputFeePpk,
			Version: seed.Version,
		})
	}
	for _, seed := range authKeysets.Keysets {
		if len(filterUnits) > 0 {
			if !slices.Contains(filterUnits, seed.Unit) {
				continue
			}
		}

		keysetMap[seed.Unit] = append(keysetMap[seed.Unit], templates.KeysetData{
			Id:      seed.Id,
			Active:  seed.Active,
			Unit:    seed.Unit,
			Fees:    seed.InputFeePpk,
			Version: seed.Version,
		})
	}

	// order the keysets by version
	for unit, ranges := range keysetMap {
		sort.Slice(ranges, func(i, j int) bool { return ranges[i].Version > ranges[j].Version })
		keysetMap[unit] = ranges
	}

	// create ordered list of units
	var orderedUnits []string
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
	milillisatBalance, err := a.mint.LightningBackend.WalletBalance()
	if err != nil {
		return 0, fmt.Errorf("a.mint.LightningBackend.WalletBalance(). %w", err)
	}
	return milillisatBalance, nil
}

func (a *adminHandler) EcashBalance(since time.Time) (templates.Balance, error) {
	proofsTotalAmountValue := uint64(0)
	proofsTotalQuantityValue := uint64(0)

	errgroup := errgroup.Group{}

	errgroup.Go(func() error {
		proofsReserve, err := a.mint.MintDB.GetProofsCountByKeyset(time.Unix(0, 0))
		if err != nil {
			return fmt.Errorf("a.mint.MintDB.GetProofsInventory(time.Unix(0, 0)). %w", err)
		}

		for _, val := range proofsReserve {
			proofsTotalAmountValue += val.TotalAmount
			proofsTotalQuantityValue += val.Count
		}
		return nil
	})

	blindSigsTotalAmountValue := uint64(0)
	blindSigsTotalQuantityValue := uint64(0)

	errgroup.Go(func() error {
		blindSigsReserve, err := a.mint.MintDB.GetBlindSigsCountByKeyset(time.Unix(0, 0))
		if err != nil {
			return fmt.Errorf("a.mint.MintDB.GetBlindSigsInventory(time.Unix(0, 0)). %w", err)
		}

		for _, val := range blindSigsReserve {
			blindSigsTotalAmountValue += val.TotalAmount
			blindSigsTotalQuantityValue += val.Count
		}
		return nil
	})
	err := errgroup.Wait()
	if err != nil {
		return templates.Balance{}, fmt.Errorf("errgroup.Wait(). %w", err)
	}

	neededBalance := blindSigsTotalAmountValue - proofsTotalAmountValue

	ratioProofSigAmountSats := (float64(proofsTotalAmountValue) / float64(blindSigsTotalAmountValue)) * 100

	return templates.Balance{
		ProofsAmount:      proofsTotalAmountValue,
		ProofsQuantity:    proofsTotalQuantityValue,
		BlindSigsAmount:   blindSigsTotalAmountValue,
		BlindSigsQuantity: blindSigsTotalQuantityValue,
		NeededBalance:     neededBalance,
		Ratio:             ratioProofSigAmountSats,
	}, nil

}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getTotalBalanceBlindSignaturesByTime(until time.Time) (uint64, error) {
	panic("still not implemented")
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getMintRequestByTime(since time.Time) (uint64, error) {
	panic("still not implemented")
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getMeltRequestByTime(since time.Time) (uint64, error) {
	panic("still not implemented")
}

//nolint:unused // Placeholder for future implementation
func (a *adminHandler) getLogs(until time.Time) (uint64, error) {
	panic("still not implemented")
}

func (a *adminHandler) getProofsCountByKeyset(since time.Time) (map[string]database.ProofsCountByKeyset, error) {
	return a.mint.MintDB.GetProofsCountByKeyset(since)
}
