package mint

import (
	"github.com/lescuer97/nutmix/api/cashu"
)

type SeedType struct {
	Version int
	Active  bool
	Unit    cashu.Unit
}

func CheckForInactiveSeeds(seeds []cashu.Seed) ([]SeedType, error) {

	seedTypes := make(map[cashu.Unit]SeedType)

	for _, seed := range seeds {
		unit, err := cashu.UnitFromString(seed.Unit)

		if err != nil {
			return nil, err
		}
		if seed.Version > seedTypes[unit].Version {

			seedTypes[unit] = SeedType{
				Version: seed.Version,
				Active:  seed.Active,
				Unit:    unit,
			}
		}

	}
	inactiveSeeds := make([]SeedType, 0)

	for _, seedType := range seedTypes {
		if !seedType.Active {
			inactiveSeeds = append(inactiveSeeds, seedType)
		}
	}

	return inactiveSeeds, nil
}
