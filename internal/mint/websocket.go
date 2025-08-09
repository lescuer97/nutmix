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
	proofChans := []chan cashu.Proof{}
	mintRequestChans := []chan cashu.MintRequestDB{}
	meltRequestChans := []chan cashu.MeltRequestDB{}
	for key, proofWatchArray := range o.Proofs {
		for i, proofWatch := range proofWatchArray {
			if proofWatch.SubId == subId {
				newArray := slices.Delete(proofWatchArray, i, i+1)
				o.Proofs[key] = newArray
				proofChans = append(proofChans, proofWatch.Channel)
			}
		}
	}
	for key, mintWatchArray := range o.MintQuote {
		for i, mintWatch := range mintWatchArray {
			if mintWatch.SubId == subId {
				newArray := slices.Delete(mintWatchArray, i, i+1)
				o.MintQuote[key] = newArray
				mintRequestChans = append(mintRequestChans, mintWatch.Channel)
			}
		}
	}
	for key, meltWatchArray := range o.MeltQuote {
		for i, meltWatch := range meltWatchArray {
			if meltWatch.SubId == subId {
				newArray := slices.Delete(meltWatchArray, i, i+1)
				o.MeltQuote[key] = newArray
				meltRequestChans = append(meltRequestChans, meltWatch.Channel)
			}
		}
	}
	o.Unlock()
	for i := range proofChans{
		close(proofChans[i])
	}
	for i := range mintRequestChans{
		close(mintRequestChans[i])
	}
	for i := range meltRequestChans{
		close(meltRequestChans[i])
	}
}

func (o *Observer) SendProofsEvent(proofs cashu.Proofs) {
	o.Lock()
	defer o.Unlock()


	for _, proof := range proofs {
		watchArray, exists := o.Proofs[proof.Y]
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
