package mint

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lescuer97/nutmix/api/cashu"
)

type ProofWatchChannel struct {
	Channel chan cashu.Proof
	SubId   string
}

type MintQuoteChannel struct {
	Channel chan cashu.MintRequestDB
	SubId   string
}

type MeltQuoteChannel struct {
	Channel chan cashu.MeltRequestDB
	SubId   string
}

type Observer struct {
	Proofs    map[string][]ProofWatchChannel
	MintQuote map[string][]MintQuoteChannel
	MeltQuote map[string][]MeltQuoteChannel
	sync.Mutex
}

func (o *Observer) AddProofWatch(y string, proofChan ProofWatchChannel) {
	o.Lock()
	defer o.Unlock()
	val, exists := o.Proofs[y]

	if exists {
		val = append(val, proofChan)
		o.Proofs[y] = val
	} else {
		o.Proofs[y] = []ProofWatchChannel{proofChan}
	}
}
func (o *Observer) AddMintWatch(quote string, mintChan MintQuoteChannel) {
	o.Lock()
	defer o.Unlock()
	val, exists := o.MintQuote[quote]

	if exists {
		val = append(val, mintChan)
		o.MintQuote[quote] = val
	} else {
		o.MintQuote[quote] = []MintQuoteChannel{mintChan}
	}
}
func (o *Observer) AddMeltWatch(quote string, meltChan MeltQuoteChannel) {
	o.Lock()
	defer o.Unlock()
	val, exists := o.MeltQuote[quote]

	if exists {
		val = append(val, meltChan)
		o.MeltQuote[quote] = val
	} else {
		o.MeltQuote[quote] = []MeltQuoteChannel{meltChan}
	}
}

func (o *Observer) RemoveWatch(subId string) {
	o.Lock()
	proofChans := make(map[chan cashu.Proof]struct{})
	mintRequestChans := make(map[chan cashu.MintRequestDB]struct{})
	meltRequestChans := make(map[chan cashu.MeltRequestDB]struct{})

	for key, proofWatchArray := range o.Proofs {
		kept := proofWatchArray[:0]
		for _, proofWatch := range proofWatchArray {
			if proofWatch.SubId == subId {
				proofChans[proofWatch.Channel] = struct{}{}
				continue
			}
			kept = append(kept, proofWatch)
		}
		if len(kept) == 0 {
			delete(o.Proofs, key)
			continue
		}
		o.Proofs[key] = kept
	}

	for key, mintWatchArray := range o.MintQuote {
		kept := mintWatchArray[:0]
		for _, mintWatch := range mintWatchArray {
			if mintWatch.SubId == subId {
				mintRequestChans[mintWatch.Channel] = struct{}{}
				continue
			}
			kept = append(kept, mintWatch)
		}
		if len(kept) == 0 {
			delete(o.MintQuote, key)
			continue
		}
		o.MintQuote[key] = kept
	}

	for key, meltWatchArray := range o.MeltQuote {
		kept := meltWatchArray[:0]
		for _, meltWatch := range meltWatchArray {
			if meltWatch.SubId == subId {
				meltRequestChans[meltWatch.Channel] = struct{}{}
				continue
			}
			kept = append(kept, meltWatch)
		}
		if len(kept) == 0 {
			delete(o.MeltQuote, key)
			continue
		}
		o.MeltQuote[key] = kept
	}
	o.Unlock()
	for proofChan := range proofChans {
		close(proofChan)
	}
	for mintRequestChan := range mintRequestChans {
		close(mintRequestChan)
	}
	for meltRequestChan := range meltRequestChans {
		close(meltRequestChan)
	}
}

func (o *Observer) SendProofsEvent(proofs cashu.Proofs) {
	o.Lock()
	defer o.Unlock()

	for _, proof := range proofs {
		watchArray, exists := o.Proofs[proof.Y.ToHex()]
		if exists {
			for _, v := range watchArray {
				v.Channel <- proof
			}
		}
	}
}

func (o *Observer) SendMeltEvent(melt cashu.MeltRequestDB) {
	o.Lock()
	watchArray, exists := o.MeltQuote[melt.Quote]
	o.Unlock()
	if exists {
		for _, v := range watchArray {
			v.Channel <- melt
		}
	}
}

func (o *Observer) SendMintEvent(mint cashu.MintRequestDB) {
	o.Lock()
	watchArray, exists := o.MintQuote[mint.Quote]
	o.Unlock()
	if exists {
		for _, v := range watchArray {
			v.Channel <- mint
		}
	}
}

func SendJson(conn *websocket.Conn, content any) error {
	contentToSend, err := json.Marshal(content)
	if err != nil {
		return err
	}

	err = conn.WriteMessage(websocket.TextMessage, contentToSend)
	if err != nil {
		return err
	}

	return nil
}
