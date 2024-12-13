package admin

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	// "github.com/lescuer97/nutmix/api/boltz"
	m "github.com/lescuer97/nutmix/internal/mint"
	// "github.com/lightningnetwork/lnd/zpay32"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"


	"github.com/lightningnetwork/lnd/zpay32"
)

const endpoint = "<Boltz API endpoint to use>"
const invoice = "<invoice that should be paid>"

const LiquidCoinType=1776 

func LightningToLiquidSwap(network *chaincfg.Params) error {

    // create public key from mint_privkey
    mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
    if mint_privkey == "" {
        return fmt.Errorf("Mint private key not available")
    }
	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
        return fmt.Errorf("hex.DecodeString(mint_privkey). %w", err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

    masterKey, err := m.MintPrivateKeyToBip32(parsedPrivateKey)
	if err != nil {
        return fmt.Errorf("hex.DecodeString(mint_privkey). %w", err)
	}

	// // path m/0' for liquid
    liquidKey, err := masterKey.NewChildKey(hdkeychain.HardenedKeyStart + LiquidCoinType)

	if err != nil {
		return  err
	}
	
    // path m/0'/0' for sat
	unitPath, err := liquidKey.NewChildKey(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return  err
	}

    index := uint32(1)
	// path m/0'/0'/index'
	receiveKey, err := unitPath.NewChildKey(hdkeychain.HardenedKeyStart + index)
	if err != nil {
		return err
	}

    log.Printf("\n receiveKey %+v", receiveKey)

    _ , err = boltz.NewClient("server")
	if err != nil {
		return fmt.Errorf(`boltz.NewClient("server"). %w`, err)
	}

    // request swap 
 //    swap, err := client.PostSwapReverse(boltz.ReverseRequest{
 //        ClaimPublicKey: receiveKey.PublicKey().Key,
	// 	From:            boltz.CurrencyBtc,
	// 	To:              boltz.CurrencyBtc,
	// 	RefundPublicKey: keys.PubKey().SerializeCompressed(),
	// 	Invoice:         invoice,
	// })
	// if err != nil {
	// 	return fmt.Errorf("Could not create swap: %s", err)
	// }

    // check scripts of the tapcripts to make sure boltz is not cheating


    // verify boltz address


    // store pubkey derivation and save swap info

    // check for update


    // Set the bolt11 invoice you wish to pay

 //    boltzApi := &boltz.Boltz{URL: endpoint}
 //    // breez-sdk-liquid-go.boltzApi
	//
 //    // brez
 //    swap, err := boltzApi.CreateSwap(boltz.CreateSwapRequest{
	// 	From:            boltz.CurrencyBtc,
	// 	To:              boltz.CurrencyBtc,
	// 	RefundPublicKey: keys.PubKey().SerializeCompressed(),
	// 	Invoice:         invoice,
	// })
	// if err != nil {
	// 	return fmt.Errorf("Could not create swap: %s", err)
	// }
 //    boltzPubKey, err := btcec.ParsePubKey(swap.ClaimPublicKey)
	// if err != nil {
	// 	return err
	// }
	//
	// tree := swap.SwapTree.Deserialize()
	// if err := tree.Init(false, keys, boltzPubKey); err != nil {
	// 	return err
	// }
	//
	// decodedInvoice, err := zpay32.Decode(invoice, network)
	// if err != nil {
	// 	return fmt.Errorf("could not decode swap invoice: %s", err)
	// }
	//
	// // Check the scripts of the Taptree to make sure Boltz is not cheating
	// if err := tree.Check(false, swap.TimeoutBlockHeight, decodedInvoice.PaymentHash[:]); err != nil {
	// 	return err
	// }
	//
	// // Verify that Boltz is giving us the correct address
	// if err := tree.CheckAddress(swap.Address, network, nil); err != nil {
	// 	return err
	// }

    return nil
}

func CheckLightningToLiquidSwap(swapId string) error {
    // destination := "<bolt11 invoice>"
    // prepareRequest := breez_sdk_liquid.PrepareSendRequest{
    //     Destination: destination,
    // }
    // prepareResponse, err := sdk.PrepareSendPayment(prepareRequest)
    // if err != nil {
    //     return fmt.Errorf("sdk.PrepareSendPayment(prepareRequest). %w", err)
    // }
    // 
    // sendFeesSat := prepareResponse.FeesSat
    // log.Printf("Fees: %v sats", sendFeesSat)
 //    keys, err := btcec.NewPrivateKey()
	// if err != nil {
	// 	return err
	// }
 //    boltzApi := &boltz.Boltz{URL: endpoint}
	//
 //    swap, err := boltzApi.CreateSwap(boltz.CreateSwapRequest{
	// 	From:            boltz.CurrencyBtc,
	// 	To:              boltz.CurrencyBtc,
	// 	RefundPublicKey: keys.PubKey().SerializeCompressed(),
	// 	Invoice:         invoice,
	// })
	// if err != nil {
	// 	return fmt.Errorf("Could not create swap: %s", err)
	// }
 //    boltzPubKey, err := btcec.ParsePubKey(swap.ClaimPublicKey)
	// if err != nil {
	// 	return err
	// }
	//
	// tree := swap.SwapTree.Deserialize()
	// if err := tree.Init(false, keys, boltzPubKey); err != nil {
	// 	return err
	// }
	//
	// decodedInvoice, err := zpay32.Decode(invoice, network)
	// if err != nil {
	// 	return fmt.Errorf("could not decode swap invoice: %s", err)
	// }
	//
	// // Check the scripts of the Taptree to make sure Boltz is not cheating
	// if err := tree.Check(false, swap.TimeoutBlockHeight, decodedInvoice.PaymentHash[:]); err != nil {
	// 	return err
	// }
	//
	// // Verify that Boltz is giving us the correct address
	// if err := tree.CheckAddress(swap.Address, network, nil); err != nil {
	// 	return err
	// }

    return nil
}
func LiquidToLightningSwap(mint m.Mint) error {

    // create public key from mint_privkey
    mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
    if mint_privkey == "" {
        return fmt.Errorf("Mint private key not available")
    }
	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
        return fmt.Errorf("hex.DecodeString(mint_privkey). %w", err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

    masterKey, err := m.MintPrivateKeyToBip32(parsedPrivateKey)
	if err != nil {
        return fmt.Errorf("hex.DecodeString(mint_privkey). %w", err)
	}

	// // path m/0' for liquid
    liquidKey, err := masterKey.NewChildKey(hdkeychain.HardenedKeyStart + LiquidCoinType)

	if err != nil {
		return  err
	}
	
    // path m/0'/0' for sat
	unitPath, err := liquidKey.NewChildKey(hdkeychain.HardenedKeyStart + 0)
	if err != nil {
		return  err
	}

    index := uint32(1)
	// path m/0'/0'/index'
	receiveKey, err := unitPath.NewChildKey(hdkeychain.HardenedKeyStart + index)
	if err != nil {
		return err
	}

    pubkey := hex.EncodeToString(receiveKey.PublicKey().Key)

    log.Printf("\n receiveKey %+v", receiveKey)

    client , err := boltz.NewClient("server")
	if err != nil {
		return fmt.Errorf(`boltz.NewClient("server"). %w`, err)
	}

    lightningres, err := mint.LightningBackend.RequestInvoice(1000)
	if err != nil {
		return fmt.Errorf(`mint.LightningBackend.RequestInvoice(1000). %w`, err)
	}
    // boltz.
    // request swap 
    swapResp, err := client.PostSwapSubmarine(context.Background(), boltz.SubmarineRequest{
		From:            "L-BTC",
		To:              "BTC",
		RefundPublicKey: &pubkey,
		Invoice:         &lightningres.PaymentRequest,
	})

	if err != nil {
		return fmt.Errorf("client.PostSwapSubmarine(boltz.SubmarineRequest: %s", err)
	}

    body, err := io.ReadAll(swapResp.Body)
    if err != nil {
		return fmt.Errorf("ioutil.ReadAll(swapResp.Body): %s", err)
    }
    defer swapResp.Body.Close()

    var swap boltz.SubmarineResponse


    err = json.Unmarshal(body,&swap)
	if err != nil {
		return fmt.Errorf("json.Unmarshal(body,&swap ): %s", err)
	}


    if boltzPubKeyString := swap.ClaimPublicKey; boltzPubKeyString == nil {
		return fmt.Errorf("No available ClaimPubkey: %+v", swap)
	}

    boltzKeyByte, err := hex.DecodeString(*swap.ClaimPublicKey)
	if err != nil {
		return fmt.Errorf("hex.DecodeString(*swap.ClaimPublicKey). %w", err)
	}

    boltzPubKey, err := btcec.ParsePubKey(boltzKeyByte)
	if err != nil {
		return fmt.Errorf("btcec.ParsePubKey(boltzKeyByte). %w", err)
	}

    if swapTree := swap.SwapTree; swapTree == nil {
		return fmt.Errorf("No SwapTree: %+v", swap)
	}

	// tree := *swap.SwapTree
 //    tree.
	// if err := tree.Init(false, keys, boltzPubKey); err != nil {
	// 	return err
	// }

	decodedInvoice, err := zpay32.Decode(invoice, mint.LightningBackend.GetNetwork())
	if err != nil {
		return fmt.Errorf("could not decode swap invoice: %s", err)
	}

	// Check the scripts of the Taptree to make sure Boltz is not cheating
	if err := tree.Check(false, swap.TimeoutBlockHeight, decodedInvoice.PaymentHash[:]); err != nil {
		return err
	}

	// Verify that Boltz is giving us the correct address
	if err := tree.CheckAddress(swap.Address, network, nil); err != nil {
		return err
	}


    // check scripts of the tapcripts to make sure boltz is not cheating


    // verify boltz address


    // store pubkey derivation and save swap info

    // check for update


    // Set the bolt11 invoice you wish to pay

	// tree := swap.SwapTree.Deserialize()
	// if err := tree.Init(false, keys, boltzPubKey); err != nil {
	// 	return err
	// }
	//
	// decodedInvoice, err := zpay32.Decode(invoice, network)
	// if err != nil {
	// 	return fmt.Errorf("could not decode swap invoice: %s", err)
	// }
	//
	// // Check the scripts of the Taptree to make sure Boltz is not cheating
	// if err := tree.Check(false, swap.TimeoutBlockHeight, decodedInvoice.PaymentHash[:]); err != nil {
	// 	return err
	// }
	//
	// // Verify that Boltz is giving us the correct address
	// if err := tree.CheckAddress(swap.Address, network, nil); err != nil {
	// 	return err
	// }

    return nil
}

func CheckLiquidToLightningSwap(swapId string) error {
    // destination := "<bolt11 invoice>"
    // prepareRequest := breez_sdk_liquid.PrepareSendRequest{
    //     Destination: destination,
    // }
    // prepareResponse, err := sdk.PrepareSendPayment(prepareRequest)
    // if err != nil {
    //     return fmt.Errorf("sdk.PrepareSendPayment(prepareRequest). %w", err)
    // }
    // 
    // sendFeesSat := prepareResponse.FeesSat
    // log.Printf("Fees: %v sats", sendFeesSat)
 //    keys, err := btcec.NewPrivateKey()
	// if err != nil {
	// 	return err
	// }
 //    boltzApi := &boltz.Boltz{URL: endpoint}
	//
 //    swap, err := boltzApi.CreateSwap(boltz.CreateSwapRequest{
	// 	From:            boltz.CurrencyBtc,
	// 	To:              boltz.CurrencyBtc,
	// 	RefundPublicKey: keys.PubKey().SerializeCompressed(),
	// 	Invoice:         invoice,
	// })
	// if err != nil {
	// 	return fmt.Errorf("Could not create swap: %s", err)
	// }
 //    boltzPubKey, err := btcec.ParsePubKey(swap.ClaimPublicKey)
	// if err != nil {
	// 	return err
	// }
	//
	// tree := swap.SwapTree.Deserialize()
	// if err := tree.Init(false, keys, boltzPubKey); err != nil {
	// 	return err
	// }
	//
	// decodedInvoice, err := zpay32.Decode(invoice, network)
	// if err != nil {
	// 	return fmt.Errorf("could not decode swap invoice: %s", err)
	// }
	//
	// // Check the scripts of the Taptree to make sure Boltz is not cheating
	// if err := tree.Check(false, swap.TimeoutBlockHeight, decodedInvoice.PaymentHash[:]); err != nil {
	// 	return err
	// }
	//
	// // Verify that Boltz is giving us the correct address
	// if err := tree.CheckAddress(swap.Address, network, nil); err != nil {
	// 	return err
	// }

    return nil
}
