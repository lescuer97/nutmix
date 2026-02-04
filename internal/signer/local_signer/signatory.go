package localsigner

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/lescuer97/nutmix/api/cashu"
	"golang.org/x/text/unicode/norm"
)

const PeanutUTF8 = uint32(129372)

func keyDerivation(version uint, unit cashu.Unit) string {
	unitInteger := parseUnitToIntegerReference(unit.String())
	return fmt.Sprintf("%v'/%v'/%v'", PeanutUTF8, unitInteger, version)
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
