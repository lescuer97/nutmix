package localsigner

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"

	"github.com/lescuer97/nutmix/api/cashu"
	"golang.org/x/text/unicode/norm"
)

const PeanutUTF8 = uint32(129372)

func keyDerivation(version uint, unit cashu.Unit) string {
	unitInteger := parseUnitToIntegerReference(unit.String())
	return fmt.Sprintf("%v'/%v'/%v'", PeanutUTF8, uint32(unitInteger), version)
}

func parseUnitToIntegerReference(unit string) uint32 {
	unit = unitNormalization(unit)
	unitSha256 := sha256.Sum256([]byte(unit))
	unitInteger := binary.BigEndian.Uint32(unitSha256[:4])
	return unitInteger &^ (1 << 31)
}

func unitNormalization(unit string) string {
	// Remove leading and trailing ASCII whitespace characters (space, tab, carriage return, line feed).
	unitStr := strings.TrimSpace(unit)
	//  Apply Unicode Normalization Form C (NFC).
	unitStr = norm.NFC.String(unitStr)
	//  Convert the normalized string to uppercase using Unicode-aware semantics
	return strings.ToUpper(unitStr)

}

type keysetAmounts = map[uint64]int

func orderAndTransformAmounts(amounts []uint64) keysetAmounts {
	// Sort the amounts
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] < amounts[j] })

	// Transform to KeysetAmounts
	keysetAmounts := make(keysetAmounts, len(amounts))
	for index, amount := range amounts {
		keysetAmounts[amount] = index
	}

	return keysetAmounts
}
