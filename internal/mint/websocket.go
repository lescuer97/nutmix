package mint

import (
	"encoding/json"
	"slices"
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
	sync.Mutex
	// the string is the filters from the websockets
	Proofs    map[string][]ProofWatchChannel
	MintQuote map[string][]MintQuoteChannel
	MeltQuote map[string][]MeltQuoteChannel
}

func (o *Observer) AddProofWatch(y string, proofChan ProofWatchChannel) {
	o.Lock()
	val, exists := o.Proofs[y]

	if exists {
		val = append(val, proofChan)
		o.Proofs[y] = val
	} else {
		o.Proofs[y] = []ProofWatchChannel{proofChan}
	}
	o.Unlock()
}
func (o *Observer) AddMintWatch(quote string, mintChan MintQuoteChannel) {
	o.Lock()
	val, exists := o.MintQuote[quote]

	if exists {
		val = append(val, mintChan)
		o.MintQuote[quote] = val
	} else {
		o.MintQuote[quote] = []MintQuoteChannel{mintChan}
	}
	o.Unlock()
}
func (o *Observer) AddMeltWatch(quote string, meltChan MeltQuoteChannel) {
	o.Lock()
	val, exists := o.MeltQuote[quote]

	if exists {
		val = append(val, meltChan)
		o.MeltQuote[quote] = val
	} else {
		o.MeltQuote[quote] = []MeltQuoteChannel{meltChan}
	}
	o.Unlock()
}

func (o *Observer) RemoveWatch(subId string) {
	o.Lock()
	for key, proofWatchArray := range o.Proofs {
		for i, proofWatch := range proofWatchArray {
			if proofWatch.SubId == subId {
				newArray := slices.Delete(proofWatchArray, i, i+1)
				o.Proofs[key] = newArray
				close(proofWatch.Channel)
			}
		}
	}
	for key, mintWatchArray := range o.MintQuote {
		for i, mintWatch := range mintWatchArray {
			if mintWatch.SubId == subId {
				newArray := slices.Delete(mintWatchArray, i, i+1)
				o.MintQuote[key] = newArray
				close(mintWatch.Channel)
			}
		}
	}
	for key, meltWatchArray := range o.MeltQuote {
		for i, meltWatch := range meltWatchArray {
			if meltWatch.SubId == subId {
				newArray := slices.Delete(meltWatchArray, i, i+1)
				o.MeltQuote[key] = newArray
				close(meltWatch.Channel)
			}
		}
	}
	o.Unlock()
}

func (o *Observer) SendProofsEvent(proofs cashu.Proofs) {
	o.Lock()
	for _, proof := range proofs {
		watchArray, exists := o.Proofs[proof.Y]
		if exists {
			for _, v := range watchArray {
				v.Channel <- proof
			}
		}
	}
	o.Unlock()
}

func (o *Observer) SendMeltEvent(melt cashu.MeltRequestDB) {
	o.Lock()
	watchArray, exists := o.MeltQuote[melt.Quote]
	if exists {
		for _, v := range watchArray {
			v.Channel <- melt
		}
	}
	o.Unlock()
}

func (o *Observer) SendMintEvent(mint cashu.MintRequestDB) {
	o.Lock()
	watchArray, exists := o.MintQuote[mint.Quote]
	if exists {
		for _, v := range watchArray {
			v.Channel <- mint
		}
	}
	o.Unlock()
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
