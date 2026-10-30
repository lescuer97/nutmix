//nolint:exhaustruct // Fixtures specify only fields relevant to the lifecycle scenario.
package postgresql

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/lescuer97/nutmix/api/cashu"
)

func lifecycleSignature(n byte) cashu.RecoverSigDB {
	return cashu.RecoverSigDB{
		B_: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{n}).PubKey()},
		C_: cashu.WrappedPublicKey{PublicKey: secp256k1.PrivKeyFromBytes([]byte{n + 1}).PubKey()},
		Id: "keyset", Amount: 1, CreatedAt: 123,
	}
}

type lifecycleRows struct { //nolint:govet // Test-double field order favors readability over packing.
	pgx.Rows
	states []string
	index  int
	err    error
}

func (r *lifecycleRows) Next() bool { r.index++; return r.index < len(r.states) }
func (r *lifecycleRows) Close()     {}
func (r *lifecycleRows) Err() error { return r.err }
func (r *lifecycleRows) Scan(dest ...any) error {
	value, ok := dest[0].(*string)
	if !ok {
		return errors.New("unexpected scan target")
	}
	*value = r.states[r.index]
	return nil
}

type lifecycleBatchResults struct {
	pgx.BatchResults
	err error
}

func (r lifecycleBatchResults) Close() error { return r.err }

type lifecycleTx struct { //nolint:govet // Test-double field order favors readability over packing.
	pgx.Tx
	locked   []string
	states   []string
	batch    *pgx.Batch
	queryErr error
	batchErr error
	lockErr  error
}

func (tx *lifecycleTx) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	if !strings.Contains(sql, "blind_messages WHERE blinded_message = ANY($1)") {
		return nil, errors.New("unexpected availability query")
	}
	return &lifecycleRows{states: tx.states, index: -1, err: tx.queryErr}, nil
}

func (tx *lifecycleTx) SendBatch(_ context.Context, batch *pgx.Batch) pgx.BatchResults {
	if strings.Contains(batch.QueuedQueries[0].SQL, "pg_advisory_xact_lock") {
		for _, query := range batch.QueuedQueries {
			key, ok := query.Arguments[0].(string)
			if !ok {
				return lifecycleBatchResults{err: errors.New("unexpected lock key")}
			}
			tx.locked = append(tx.locked, key)
		}
		return lifecycleBatchResults{err: tx.lockErr}
	}
	tx.batch = batch
	return lifecycleBatchResults{err: tx.batchErr}
}

func TestBlindMessagePersistenceValidatesBeforeWriting(t *testing.T) {
	valid := lifecycleSignature(1)
	for _, tt := range []struct { //nolint:govet // Table layout favors readable fixtures over packing.
		name string
		sigs []cashu.RecoverSigDB
		want error
	}{
		{"empty", nil, nil},
		{"missing message", []cashu.RecoverSigDB{{C_: valid.C_, Amount: 1, Id: valid.Id}}, cashu.ErrInvalidBlindMessage},
		{"missing signature", []cashu.RecoverSigDB{{B_: valid.B_, Amount: 1, Id: valid.Id}}, cashu.ErrInvalidBlindMessage},
		{"zero amount", []cashu.RecoverSigDB{{B_: valid.B_, C_: valid.C_, Id: valid.Id}}, cashu.ErrInvalidBlindMessage},
		{"missing keyset", []cashu.RecoverSigDB{{B_: valid.B_, C_: valid.C_, Amount: 1}}, cashu.ErrInvalidBlindMessage},
		{"incomplete dleq", []cashu.RecoverSigDB{{B_: valid.B_, C_: valid.C_, Amount: 1, Id: valid.Id, Dleq: &cashu.BlindSignatureDLEQ{}}}, cashu.ErrInvalidBlindMessage},
		{"duplicates", []cashu.RecoverSigDB{valid, valid}, cashu.ErrRepeatedOutput},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tx := &lifecycleTx{}
			err := (Postgresql{}).SaveRestoreSigs(tx, tt.sigs)
			if !errors.Is(err, tt.want) || len(tx.locked) != 0 || tx.batch != nil {
				t.Fatalf("error=%v, locks=%v, batch=%v", err, tx.locked, tx.batch)
			}
		})
	}
}

func TestBlindMessagePersistenceLocksAndLinksInBatch(t *testing.T) {
	sigs := []cashu.RecoverSigDB{lifecycleSignature(2), lifecycleSignature(1)}
	sigs[0].MintQuote = "mint-quote"
	sigs[0].Amount = math.MaxUint64
	sigs[0].Dleq = &cashu.BlindSignatureDLEQ{E: secp256k1.PrivKeyFromBytes([]byte{1}), S: secp256k1.PrivKeyFromBytes([]byte{2})}
	tx := &lifecycleTx{}
	if err := (Postgresql{}).SaveRestoreSigs(tx, sigs); err != nil {
		t.Fatal(err)
	}
	want := []string{sigs[0].B_.ToHex(), sigs[1].B_.ToHex()}
	sort.Strings(want)
	if !reflect.DeepEqual(tx.locked, want) || tx.batch == nil || tx.batch.Len() != len(sigs) {
		t.Fatalf("locks=%v batch=%v", tx.locked, tx.batch)
	}
	for i, query := range tx.batch.QueuedQueries {
		if !strings.Contains(query.SQL, "INSERT INTO blind_signatures") ||
			!strings.Contains(query.SQL, "INSERT INTO blind_messages") ||
			query.Arguments[7] != sigs[i].MintQuote || query.Arguments[5] != sigs[i].Amount {
			t.Fatalf("incorrect message/signature association: %+v", query)
		}
	}
}

func TestBlindMessagePersistenceRejectsExistingOutputsAndErrors(t *testing.T) {
	for _, tt := range []struct { //nolint:govet // Table layout favors readable fixtures over packing.
		name     string
		states   []string
		queryErr error
		batchErr error
		lockErr  error
		want     error
	}{
		{name: "pending", states: []string{"PENDING"}, want: cashu.ErrOutputPending},
		{name: "signed", states: []string{"SIGNED"}, want: cashu.ErrBlindMessageAlreadySigned},
		{name: "row error", queryErr: context.Canceled, want: context.Canceled},
		{name: "batch error", batchErr: context.Canceled, want: context.Canceled},
		{name: "lock error", lockErr: context.Canceled, want: context.Canceled},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tx := &lifecycleTx{states: tt.states, queryErr: tt.queryErr, batchErr: tt.batchErr, lockErr: tt.lockErr}
			if err := (Postgresql{}).SaveRestoreSigs(tx, []cashu.RecoverSigDB{lifecycleSignature(1)}); !errors.Is(err, tt.want) {
				t.Fatalf("error=%v, want %v", err, tt.want)
			}
			if tt.batchErr == nil && tx.batch != nil {
				t.Fatal("attempted persistence after availability failure")
			}
		})
	}
}

func TestBlindMessageAmountNumericRoundTrip(t *testing.T) {
	typeMap := pgtype.NewMap()
	for _, amount := range []uint64{1, math.MaxInt64, math.MaxUint64} {
		encoded, err := typeMap.Encode(pgtype.NumericOID, pgtype.BinaryFormatCode, amount, nil)
		if err != nil {
			t.Fatal(err)
		}
		var decoded uint64
		if err := typeMap.Scan(pgtype.NumericOID, pgtype.BinaryFormatCode, encoded, &decoded); err != nil || decoded != amount {
			t.Fatalf("amount=%d decoded=%d err=%v", amount, decoded, err)
		}
	}
}
