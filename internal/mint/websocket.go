package mint

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
)

type WSStateChecker interface {
	WatchForChanges(pool *pgxpool.Pool, mint *Mint, wsConn *websocket.Conn) error
}

func GetCorrectStatusChecker(req cashu.WsRequest) WSStateChecker {
	var reqChecker WSStateChecker

	switch req.Params.Kind {
	case cashu.Bolt11MintQuote:
		reqChecker = MintStatusChecker{
			statuses: req.Params.Filters,
			subId:    req.Params.SubId,
			id:       req.Id,
		}
	case cashu.Bolt11MeltQuote:
		reqChecker = MeltStatusChecker{
			statuses: req.Params.Filters,
			subId:    req.Params.SubId,
			id:       req.Id,
		}

	case cashu.ProofStateWs:
		reqChecker = ProofStatusChecker{
			proofs: req.Params.Filters,
			subId:  req.Params.SubId,
			id:     req.Id,
		}

	}
	return reqChecker

}

type MintStatusChecker struct {
	statuses []string
	subId    string
	id       int
}

func (m MintStatusChecker) checkState(pool *pgxpool.Pool, mint *Mint) ([]cashu.PostMintQuoteBolt11Response, error) {
	var mintQuotes []cashu.PostMintQuoteBolt11Response
	for _, v := range m.statuses {
		quote, err := CheckMintRequest(pool, mint, v)
		if err != nil {
			return mintQuotes, fmt.Errorf("m.CheckMintRequest(pool, mint,v ) %w", err)
		}

		mintQuotes = append(mintQuotes, quote)
	}
	return mintQuotes, nil

}
func (m MintStatusChecker) WatchForChanges(pool *pgxpool.Pool, mint *Mint, wsConn *websocket.Conn) error {
	initialMintStatus, err := m.checkState(pool, mint)
	if err != nil {
		return fmt.Errorf("m.checkState(pool, mint) %w", err)
	}
	// check if the statues are different if the are send a message back

	// store mint request statuses to check for changes later
	var storedRequests = make(map[string]cashu.PostMintQuoteBolt11Response)

	for i := 0; i < len(initialMintStatus); i++ {
		storedRequests[initialMintStatus[i].Quote] = initialMintStatus[i]

		statusNotif := cashu.WsNotification{
			JsonRpc: "2.0",
			Method:  cashu.Subcribe,
			Id:      m.id,
			Params: cashu.WebRequestParams{
				SubId:   m.subId,
				Payload: initialMintStatus[i],
			},
		}
		err = SendJson(wsConn, statusNotif)
		if err != nil {
			return fmt.Errorf("sendJson(wsConn, statusNotif). %w", err)
		}

	}

	for {

		mintRequestStatus, err := m.checkState(pool, mint)
		if err != nil {
			return fmt.Errorf("m.checkState(pool, mint) %w", err)
		}

		for i := 0; i < len(mintRequestStatus); i++ {
			if mintRequestStatus[i].State != storedRequests[mintRequestStatus[i].Quote].State {
				storedRequests[mintRequestStatus[i].Quote] = mintRequestStatus[i]
				statusNotif := cashu.WsNotification{
					JsonRpc: "2.0",
					Method:  cashu.Subcribe,
					Id:      m.id,
					Params: cashu.WebRequestParams{
						SubId:   m.subId,
						Payload: mintRequestStatus[i],
					},
				}
				err = SendJson(wsConn, statusNotif)
				if err != nil {
					return fmt.Errorf("sendJson(wsConn, statusNotif). %w", err)
				}
			}

		}

		time.Sleep(5 * time.Second)
	}
}

type MeltStatusChecker struct {
	statuses []string
	subId    string
	id       int
}

func (m MeltStatusChecker) checkState(pool *pgxpool.Pool, mint *Mint) ([]cashu.PostMeltQuoteBolt11Response, error) {
	var meltQuotes []cashu.PostMeltQuoteBolt11Response
	for _, v := range m.statuses {
		quote, err := CheckMeltRequest(pool, mint, v)
		if err != nil {
			return meltQuotes, fmt.Errorf("m.CheckMintRequest(pool, mint,v ) %w", err)
		}

		meltQuotes = append(meltQuotes, quote)
	}
	return meltQuotes, nil

}
func (m MeltStatusChecker) WatchForChanges(pool *pgxpool.Pool, mint *Mint, wsConn *websocket.Conn) error {
	meltRequestsStatus, err := m.checkState(pool, mint)
	if err != nil {
		return fmt.Errorf("m.checkState(pool, mint) %w", err)
	}
	var storedRequest = make(map[string]cashu.PostMeltQuoteBolt11Response)

	for i := 0; i < len(meltRequestsStatus); i++ {
		// if status changed send info back to the websocket
		storedRequest[meltRequestsStatus[i].Quote] = meltRequestsStatus[i]
		statusNotif := cashu.WsNotification{
			JsonRpc: "2.0",
			Method:  cashu.Subcribe,
			Id:      m.id,
			Params: cashu.WebRequestParams{
				SubId:   m.subId,
				Payload: meltRequestsStatus[i],
			},
		}
		err = SendJson(wsConn, statusNotif)
		if err != nil {
			return fmt.Errorf("sendJson(wsConn, statusNotif). %w", err)
		}

	}

	for {

		statuses, err := m.checkState(pool, mint)
		if err != nil {
			return fmt.Errorf("m.checkState(pool, mint) %w", err)
		}

		for i := 0; i < len(statuses); i++ {
			if statuses[i].State != storedRequest[statuses[i].Quote].State {
				storedRequest[meltRequestsStatus[i].Quote] = statuses[i]
				statusNotif := cashu.WsNotification{
					JsonRpc: "2.0",
					Method:  cashu.Subcribe,
					Id:      m.id,
					Params: cashu.WebRequestParams{
						SubId:   m.subId,
						Payload: statuses[i],
					},
				}
				err = SendJson(wsConn, statusNotif)
				if err != nil {
					return fmt.Errorf("sendJson(wsConn, statusNotif). %w", err)
				}
			}

		}

		time.Sleep(5 * time.Second)
	}
}

type ProofStatusChecker struct {
	proofs []string
	subId  string
	id     int
}

func (p ProofStatusChecker) checkState(pool *pgxpool.Pool, mint *Mint) ([]cashu.CheckState, error) {
	var proofsState []cashu.CheckState
	proofsState, err := CheckProofState(pool, mint, p.proofs)

	if err != nil {
		return proofsState, fmt.Errorf("m.CheckMintRequest(pool, mint, p.proofs ) %w", err)
	}

	return proofsState, nil

}
func (m ProofStatusChecker) WatchForChanges(pool *pgxpool.Pool, mint *Mint, wsConn *websocket.Conn) error {
	proofsStateStatus, err := m.checkState(pool, mint)
	if err != nil {
		return fmt.Errorf("m.checkState(pool, mint) %w", err)
	}
	var storedProofState = make(map[string]cashu.CheckState)

	for i := 0; i < len(proofsStateStatus); i++ {
		storedProofState[proofsStateStatus[i].Y] = proofsStateStatus[i]
		statusNotif := cashu.WsNotification{
			JsonRpc: "2.0",
			Method:  cashu.Subcribe,
			Id:      m.id,
			Params: cashu.WebRequestParams{
				SubId:   m.subId,
				Payload: proofsStateStatus[i],
			},
		}
		err = SendJson(wsConn, statusNotif)
		if err != nil {
			return fmt.Errorf("sendJson(wsConn, statusNotif). %w", err)
		}

	}

	for {

		statuses, err := m.checkState(pool, mint)
		if err != nil {
			return fmt.Errorf("m.checkState(pool, mint) %w", err)
		}

		for i := 0; i < len(statuses); i++ {
			if statuses[i].State != storedProofState[statuses[i].Y].State {
				storedProofState[statuses[i].Y] = statuses[i]
				statusNotif := cashu.WsNotification{
					JsonRpc: "2.0",
					Method:  cashu.Subcribe,
					Id:      m.id,
					Params: cashu.WebRequestParams{
						SubId:   m.subId,
						Payload: statuses[i],
					},
				}
				err = SendJson(wsConn, statusNotif)
				if err != nil {
					return fmt.Errorf("sendJson(wsConn, statusNotif). %w", err)
				}
			}

		}

		time.Sleep(5 * time.Second)

	}
}

func SendJson(conn *websocket.Conn, content any) error {
	contentToSend, err := json.Marshal(content)
	if err != nil {
		return err
	}

	conn.WriteMessage(websocket.TextMessage, contentToSend)

	return nil
}
