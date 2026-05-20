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
	defer o.Unlock()

	for key, proofWatchArray := range o.Proofs {
		o.Proofs[key] = slices.DeleteFunc(proofWatchArray, func(proofWatchChan ProofWatchChannel) bool {
			if proofWatchChan.SubId == subId {
				close(proofWatchChan.Channel)
				return true
			}
			return false
		})
	}

	for key, mintWatchArray := range o.MintQuote {
		o.MintQuote[key] = slices.DeleteFunc(mintWatchArray, func(mintWatchChannel MintQuoteChannel) bool {
			if mintWatchChannel.SubId == subId {
				close(mintWatchChannel.Channel)
				return true
			}
			return false
		})
	}

	for key, meltWatchArray := range o.MeltQuote {
		o.MeltQuote[key] = slices.DeleteFunc(meltWatchArray, func(meltWatchChannel MeltQuoteChannel) bool {
			if meltWatchChannel.SubId == subId {
				close(meltWatchChannel.Channel)
				return true
			}
			return false
		})
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
	defer o.Unlock()
	if exists {
		for _, v := range watchArray {
			v.Channel <- melt
		}
	}
}

func (o *Observer) SendMintEvent(mint cashu.MintRequestDB) {
	o.Lock()
	watchArray, exists := o.MintQuote[mint.Quote]
	defer o.Unlock()
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
