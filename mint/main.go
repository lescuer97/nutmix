package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/lescuer97/cashu-v4v/cashu"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/tyler-smith/go-bip32"
)




type KeysetResponse struct {
	Id   string            `json:"id"`
	Unit string            `json:"unit"`
	Keys map[string]string `json:"keys"`
}

func orderKeysetByUnit(keysets []cashu.Keyset) map[string][]KeysetResponse {
	var typesOfUnits = make(map[string][]cashu.Keyset)

	for _, keyset := range keysets {
		if len(typesOfUnits[keyset.Unit]) == 0 {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
			continue
		} else {
			typesOfUnits[keyset.Unit] = append(typesOfUnits[keyset.Unit], keyset)
		}
	}

	res := make(map[string][]KeysetResponse)

	res["keysets"] = []KeysetResponse{}

	for _, value := range typesOfUnits {
		var keysetResponse KeysetResponse
		keysetResponse.Id = value[0].Id
		keysetResponse.Unit = value[0].Unit
		keysetResponse.Keys = make(map[string]string)

		for _, keyset := range value {
			keysetResponse.Keys[strconv.Itoa(keyset.Amount)] = hex.EncodeToString(keyset.PubKey)
		}

		res["keysets"] = append(res["keysets"], keysetResponse)
	}
	return res

}


type SwapMintMethod struct {
	Method    string `json:"method"`
	Unit      string `json:"unit"`
	MinAmount int    `json:"min_amount"`
	MaxAmount int    `json:"max_amount"`
}

type SwapMintInfo struct {
	Methods  *[]SwapMintMethod `json:"methods,omitempty"`
	Disabled bool              `json:"disabled"`
}

type GetInfoResponse struct {
	Name            string     `json:"name"`
	Version         string     `json:"version"`
	Pubkey          string     `json:"pubkey"`
	Description     string     `json:"description"`
	DescriptionLong string     `json:"description_long"`
	Contact         [][]string `json:"contact"`
	Motd            string     `json:"motd"`
	Nuts            map[string]SwapMintInfo
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

	keysets := CheckForActiveKeyset(conn)

	if len(keysets) == 0 {
		seed, err := bip32.NewSeed()

		if err != nil {
			log.Fatal("Error creating seed ", err)
		}

		// Get the current time
		currentTime := time.Now()

		// Format the time as a string
		formattedTime := currentTime.Unix()
		masterKey, err := bip32.NewMasterKey(seed)


		list_of_keys := cashu.GenerateKeysets(masterKey, cashu.PosibleKeysetValues)

		id := cashu.DeriveKeysetId(list_of_keys)

		for i, _ := range list_of_keys {
			list_of_keys[i].Id = id
		}

		newSeed := cashu.Seed{
			Seed:      seed,
			Active:    true,
			CreatedAt: formattedTime,
			Unit:      "sats",
			Id:        id,
		}

		SaveNewSeed(conn, &newSeed)

		SaveNewKeysets(conn, list_of_keys)

		if err != nil {
			log.Fatal("Error creating master key ", err)
		}

	}

	r := gin.Default()

	r.GET("/v1/keys", func(c *gin.Context) {

		keysets := CheckForActiveKeyset(conn)
		keys := orderKeysetByUnit(keysets)

		c.JSON(200, keys)

	})
	r.GET("/v1/keys/:id", func(c *gin.Context) {

		id := c.Param("id")
		keysets := CheckForKeysetById(conn, id)
		keys := orderKeysetByUnit(keysets)

		c.JSON(200, keys)

	})

	type BasicKeysetResponse struct {
		Id     string `json:"id"`
		Unit   string `json:"unit"`
		Active bool   `json:"active"`
	}

	r.GET("/v1/keysets", func(c *gin.Context) {

		seeds := GetAllSeeds(conn)
		fmt.Println("Seeds", seeds)

		keys := make(map[string][]BasicKeysetResponse)
		keys["keysets"] = []BasicKeysetResponse{}

		for _, seed := range seeds {
			keys["keysets"] = append(keys["keysets"], BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active})

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
		motd := os.Getenv("DESCRIPTION_LONG")

		email := []string{"email", os.Getenv("EMAIL")}
		nostr := []string{"nostr", os.Getenv("NOSTR")}

		contacts := [][]string{email, nostr}

		for i, contact := range contacts {
			if contact[1] == "" {
				contacts = append(contacts[:i], contacts[i+1:]...)
			}
		}

		nuts := make(map[string]SwapMintInfo)

		nuts["1"] = SwapMintInfo{
			Disabled: false,
		}

		nuts["2"] = SwapMintInfo{
			Disabled: false,
		}

		nuts["6"] = SwapMintInfo{
			Disabled: false,
		}

		response := GetInfoResponse{
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

    type PostMintQuoteBolt11Request struct {
        Amount int64 `json:"amount"`
        Unit     string `json:"unit"`
    }


    r.POST("/v1/mint/quote/bolt11", func(c *gin.Context) {
        var mintRequest PostMintQuoteBolt11Request
        c.BindJSON(&mintRequest)

        invoice :=     lnrpc.Invoice{
            Memo: "Mint request",
            Settled: false,
            Value: mintRequest.Amount,
            Receipt: make([]byte, 0),
            RPreimage: make([]byte, 0),
            RHash: make([]byte, 0),
        }
	    jsonBytes, err := lnrpc.ProtoJSONMarshalOpts.Marshal(invoice)
	if err != nil {
		fmt.Println("unable to decode response: ", err)
		return
	}

	fmt.Printf("%s\n", jsonBytes)


        
        fmt.Println("Mint request", mintRequest)

    })

	r.Run(":8080")
}
