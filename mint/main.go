package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/lescuer97/cashu-v4v/cashu"
	"github.com/lescuer97/cashu-v4v/lightning"
	"github.com/lightningnetwork/lnd/channeldb/migration_01_to_11/zpay32"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/tyler-smith/go-bip32"
    "github.com/gin-contrib/cors"
	"log"
	"os"
	"slices"
	"strconv"
	"time"
)

func orderKeysetByUnit(keysets []cashu.Keyset) (map[string][]cashu.KeysetResponse, error) {
	var typesOfUnits = make(map[string][]cashu.Keyset)

	for _, keyset := range keysets {
		if len(typesOfUnits[keyset.Unit]) == 0 {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
			continue
		} else {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
		}
	}

	res := make(map[string][]cashu.KeysetResponse)

	res["keysets"] = []cashu.KeysetResponse{}

	for _, value := range typesOfUnits {
		var keysetResponse cashu.KeysetResponse
		keysetResponse.Id = value[0].Id
		keysetResponse.Unit = value[0].Unit
		keysetResponse.Keys = make(map[string]string)

		for _, keyset := range value {
			privkey, err := bip32.B58Deserialize(keyset.PrivKey)
			if err != nil {

				return nil, err
			}
			keysetResponse.Keys[strconv.Itoa(keyset.Amount)] = hex.EncodeToString(privkey.PublicKey().Key)
		}

		res["keysets"] = append(res["keysets"], keysetResponse)
	}
	return res, nil

}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	databaseConUrl := os.Getenv("DATABASE_URL")

	conn, err := pgx.Connect(context.Background(), databaseConUrl)

	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

	defer conn.Close(context.Background())

	keysets, err := CheckForActiveKeyset(conn)

	if err != nil {
		log.Fatalf("Could not keysets: %v", err)
	}

	if len(keysets) == 0 {
		seed, err := bip32.NewSeed()

		if err != nil {
			log.Fatalf("Error creating seed: %+v ", err)
		}

		// Get the current time
		currentTime := time.Now().Unix()

		// // Format the time as a string
		masterKey, err := bip32.NewMasterKey(seed)

		list_of_keys := cashu.GenerateKeysets(masterKey, cashu.PosibleKeysetValues)

		id, err := cashu.DeriveKeysetId(list_of_keys)

		if err != nil {
			log.Fatalf("Error DeriveKeysetId: %+v ", err)
		}

		for i, _ := range list_of_keys {
			list_of_keys[i].Id = id
		}

		newSeed := cashu.Seed{
			Seed:      seed,
			Active:    true,
			CreatedAt: currentTime,
			Unit:      "sat",
			Id:        id,
		}

		err = SaveNewSeed(conn, &newSeed)

		if err != nil {
			log.Fatalf("SaveNewSeed: %v ", err)
		}

		err = SaveNewKeysets(conn, list_of_keys)

		if err != nil {
			log.Fatalf("SaveNewKeysets: %v ", err)
		}
	}

	r := gin.Default()
      r.Use(cors.Default())

	r.GET("/v1/keys", func(c *gin.Context) {

		keysets, err := CheckForActiveKeyset(conn)

		if err != nil {
			log.Fatalf("CheckForActiveKeyset: %v ", err)
			c.JSON(500, "Server side error")
			return
		}

		keys, err := orderKeysetByUnit(keysets)
		if err != nil {
			log.Printf("orderKeysetByUnit: %v ", err)
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)

	})

	r.GET("/v1/keys/:id", func(c *gin.Context) {

		id := c.Param("id")
		keysets, err := CheckForKeysetById(conn, id)
		if err != nil {
			log.Fatalf("CheckForKeysetById: %v ", err)
			c.JSON(500, "Server side error")
			return
		}
		keys, err := orderKeysetByUnit(keysets)
		if err != nil {
			log.Printf("orderKeysetByUnit: %v ", err)
			c.JSON(500, "Server side error")
			return
		}

		c.JSON(200, keys)

	})

	r.GET("/v1/keysets", func(c *gin.Context) {

		seeds, err := GetAllSeeds(conn)
		if err != nil {
			log.Fatalf("GetAllSeeds: %v ", err)
			c.JSON(500, "Server side error")
			return
		}

		keys := make(map[string][]cashu.BasicKeysetResponse)
		keys["keysets"] = []cashu.BasicKeysetResponse{}

		for _, seed := range seeds {
			keys["keysets"] = append(keys["keysets"], cashu.BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active})

		}

		c.JSON(200, keys)

	})

	r.GET("/v1/info", func(c *gin.Context) {

		seed, err := GetActiveSeed(conn)

		var pubkey string = ""

		if err != nil {
			log.Fatal("Error getting active seed: ", err)
		}

		masterKey, err := bip32.NewMasterKey(seed.Seed)

		if err != nil {
			log.Fatal("Error creating master key ", err)
		}
		pubkey = hex.EncodeToString(masterKey.PublicKey().Key)
		name := os.Getenv("NAME")
		description := os.Getenv("DESCRIPTION")
		description_long := os.Getenv("DESCRIPTION_LONG")
		motd := os.Getenv("MOTD")

		email := []string{"email", os.Getenv("EMAIL")}
		nostr := []string{"nostr", os.Getenv("NOSTR")}

		contacts := [][]string{email, nostr}

		for i, contact := range contacts {
			if contact[1] == "" {
				contacts = append(contacts[:i], contacts[i+1:]...)
			}
		}

		nuts := make(map[string]cashu.SwapMintInfo)

		nuts["1"] = cashu.SwapMintInfo{
			Disabled: false,
		}

		nuts["2"] = cashu.SwapMintInfo{
			Disabled: false,
		}

		nuts["6"] = cashu.SwapMintInfo{
			Disabled: false,
		}

		response := cashu.GetInfoResponse{
			Name:            name,
			Version:         "AwesomeGoMint/0.1",
			Pubkey:          pubkey,
			Description:     description,
			DescriptionLong: description_long,
			Motd:            motd,
			Contact:         contacts,
			Nuts:            nuts,
		}

		c.JSON(200, response)
	})

	r.POST("/v1/mint/quote/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintQuoteBolt11Request
		c.BindJSON(&mintRequest)

		fmt.Println("amount: ", mintRequest.Amount)
		if mintRequest.Amount == 0 {
			c.JSON(400, "amount missing")
			return
		}

		payReq, err := lightning.CreateMockInvoice(mintRequest.Amount, "mock invoice")
		if err != nil {
			log.Println(err)
			c.JSON(500, "Opps!, something went wrong")

		}

		randUuid, err := uuid.NewRandom()
		if err != nil {

			log.Println(fmt.Errorf("NewRamdom: %v", err))

			c.JSON(500, "Opps!, something went wrong")

		}

		postRequest := cashu.PostMintQuoteBolt11Response{
			Quote:   randUuid.String(),
			Request: payReq,
			Paid:    true,
			Expiry:  3600,
		}

		err = SaveQuoteRequest(conn, postRequest)
		if err != nil {
			log.Println(fmt.Errorf("SaveQuoteRequest: %v", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		c.JSON(200, postRequest)

	})

	r.GET("/v1/mint/quote/bolt11/:quote", func(c *gin.Context) {
		quoteId := c.Param("quote")

		quote, err := GetQuoteById(conn, quoteId)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %v", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		c.JSON(200, quote)
	})

	r.POST("/v1/mint/bolt11", func(c *gin.Context) {
		var mintRequest cashu.PostMintBolt11Request
		err = c.BindJSON(&mintRequest)
		if err != nil {
			log.Printf("Incorrect body: %+v", err)
			c.JSON(400, "Malformed body request")
		}

		quote, err := GetQuoteById(conn, mintRequest.Quote)

		if err != nil {
			log.Println(fmt.Errorf("GetQuoteById: %v", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		// check if the quote is paid
		if quote.Paid == false {
			c.JSON(400, "Quote not paid")
			return
		}

		invoice, err := zpay32.Decode(quote.Request, &chaincfg.MainNetParams)

		if err != nil {
			log.Println(fmt.Errorf("zpay32.Decode: %v", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		var amount int32 = 0
        var amounts []int32 = make([]int32, 0)

		for _, output := range mintRequest.Outputs {
			amount += output.Amount
            amounts = append(amounts, output.Amount)

		}

		amountMilsats, err := lnrpc.UnmarshallAmt(int64(amount), 0)

		if err != nil {
			log.Println(fmt.Errorf("UnmarshallAmt: %v", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		// check the amount in outputs are the same as the quote
		if int32(*invoice.MilliSat) != int32(amountMilsats) {
			c.JSON(400, "Amounts in outputs are not the same")
			return
		}

        keysets , err := GetKeysetsByAmountList(conn, amounts)

		if err != nil {
			log.Println(fmt.Errorf("GetKeysetsByAmountList: %v", err))
			c.JSON(500, "Opps!, something went wrong")
		}

		var blindedSignatures []cashu.BlindSignature
		// Create blindSignature with diffie-hellman exchange
		for _, output := range mintRequest.Outputs {

			index := slices.Index(cashu.PosibleKeysetValues, int(output.Amount))

			if index == -1 {
				c.JSON(500, "Something happened")
				return
			}


            key, err := bip32.B58Deserialize( keysets[int(output.Amount)].PrivKey)
			if err != nil {
				log.Println(fmt.Errorf("bip32.B58Deserialize: %v", err))
				c.JSON(500, "Opps!, something went wrong")
			}
            
            blindSignature, err :=  cashu.GenerateBlindSignature(key, output)

			if err != nil {
				log.Println(fmt.Errorf("GenerateBlindSignature: %v", err))
				c.JSON(500, "Opps!, something went wrong")
			}

			blindedSignatures = append(blindedSignatures, blindSignature)

		}
		// Store BlidedSignature

		c.JSON(200, cashu.PostMintBolt11Response {
            Signatures: blindedSignatures,
        })
	})

	r.Run(":8080")
}
