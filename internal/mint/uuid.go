package mint

import (
	"crypto/rand"
	"io"
	"time"

	"github.com/google/uuid"
)

func newQuoteUUID() (uuid.UUID, error) {
	return newQuoteUUIDFrom(time.Now(), rand.Reader)
}

// Kept separate to make the UUIDv7 layout testable without changing global RNG state.
func newQuoteUUIDFrom(now time.Time, entropy io.Reader) (uuid.UUID, error) {
	var id uuid.UUID
	if _, err := io.ReadFull(entropy, id[:]); err != nil {
		return uuid.Nil, err
	}
	ms := uint64(now.UnixMilli())
	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)
	id[6] = (id[6] & 0x0f) | 0x70
	id[8] = (id[8] & 0x3f) | 0x80
	return id, nil
}
