package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/tyler-smith/go-bip32"
)

type BlindedMessage struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	B_     string `json:"B_"`
}

type BlindSignature struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	C_     string `json:"C_"`
}

type Proof struct {
	Amount int32  `json:"amount"`
	Id     string `json:"id"`
	Secret string `json:"secret"`
	C_     string `json:"C_"`
}

type MintError struct {
	Detail string `json:"detail"`
	Code   int8   `json:"code"`
}

type Keyset struct {
	Id        string `json:"id"`
	Active    bool   `json:"active" db:"active"`
	Unit      string `json:"unit"`
	Amount    int    `json:"amount"`
	PubKey    []byte `json:"pub_key"`
	CreatedAt int64  `json:"created_at"`
}

type Seed struct {
	Seed      []byte
	Active    bool
	CreatedAt int64
    Unit     string
    Id       string
}

func deriveKeysetId(keysets []Keyset) string {
	concatBinaryArray := []byte{}
	for _, keyset := range keysets {
		concatBinaryArray = append(concatBinaryArray, keyset.PubKey...)
	}
	hashedKeysetId := sha256.Sum256(concatBinaryArray)
	hex := hex.EncodeToString(hashedKeysetId[:])

	return "00" + string(hex[:14])

}

func getAllSeeds(conn *pgx.Conn) []Seed {
	var seeds []Seed

	rows, err := conn.Query(context.Background(), "SELECT * FROM seeds")

	if err != nil {
		if err == pgx.ErrNoRows {
			return seeds
		}
		log.Fatal("Error checking for  seeds: ", err)
	}

	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[Seed])

	if err != nil {
		log.Fatal("Error checking for seeds: ", err)
	}

	return keysets_collect
}

func checkForActiveKeyset(conn *pgx.Conn) []Keyset {
	var keysets []Keyset

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE active")
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets
		}
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[Keyset])

	if err != nil {
		log.Fatal("Error checking for active keyset: ", err)
	}

	return keysets_collect
}

func checkForKeysetById(conn *pgx.Conn, id string) []Keyset {
	var keysets []Keyset

	rows, err := conn.Query(context.Background(), "SELECT * FROM keysets WHERE id = $1", id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return keysets
		}
		log.Fatal("Error checking for active keyset: ", err)
	}
	defer rows.Close()

	keysets_collect, err := pgx.CollectRows(rows, pgx.RowToStructByName[Keyset])

	if err != nil {
		log.Fatal("Error checking for active keyset: ", err)
	}

	return keysets_collect
}

func saveNewSeed(conn *pgx.Conn, seed *Seed) {
	_, err := conn.Exec(context.Background(), "INSERT INTO seeds (seed, active, created_at, unit, id) VALUES ($1, $2, $3, $4, $5)", seed.Seed, seed.Active, seed.CreatedAt, seed.Unit, seed.Id)
	if err != nil {
		log.Fatal("Error saving new seed: ", err)
	}
}

func saveNewKeysets(conn *pgx.Conn, keyset []Keyset) {
	for _, key := range keyset {
		_, err := conn.Exec(context.Background(), "INSERT INTO keysets (id, active, unit, amount, pubkey, created_at) VALUES ($1, $2, $3, $4, $5, $6)", key.Id, key.Active, key.Unit, key.Amount, key.PubKey, key.CreatedAt)
		if err != nil {
			log.Fatal("Error saving new keyset: ", err)
		}
	}
}

type KeysetResponse struct {
	Id   string            `json:"id"`
	Unit string            `json:"unit"`
	Keys map[string]string `json:"keys"`
}

func orderKeysetByUnit(keysets []Keyset) map[string][]KeysetResponse {

	var typesOfUnits = make(map[string][]Keyset)

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

func generateKeysets(masterKey *bip32.Key, values []int) []Keyset {
	var keysets []Keyset

	// Get the current time
	currentTime := time.Now()

	// Format the time as a string
	formattedTime := currentTime.Unix()

	for i, value := range values {
		childKey, err := masterKey.NewChildKey(uint32(i))
		if err != nil {
			log.Fatal("Error generating child key: ", err)
		}
		keyset := Keyset{
			Id:        "",
			Active:    true,
			Unit:      "sats",
			Amount:    value,
			PubKey:    childKey.PublicKey().Key,
			CreatedAt: formattedTime,
		}

		keysets = append(keysets, keyset)
	}

	return keysets
}

type SwapMintMethod struct {
    Method string `json:"method"`
    Unit string `json:"unit"`
    MinAmount int `json:"min_amount"`
    MaxAmount int `json:"max_amount"`
}

type SwapMintInfo struct {
    Methods *[]SwapMintMethod `json:"methods,omitempty"`
    Disabled bool `json:"disabled"`
}

type GetInfoResponse struct {
    Name string `json:"name"`
    Version string `json:"version"`
    Pubkey string `json:"pubkey"`
    Description string `json:"description"`
    DescriptionLong string `json:"description_long"`
    Contact [][]string `json:"contact"`
    Motd string `json:"motd"`
    Nuts map[string]SwapMintInfo
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

	keysets := checkForActiveKeyset(conn)

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


		// values for keysets with 2 over n
		newKeysetValues := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072}

		list_of_keys := generateKeysets(masterKey, newKeysetValues)

		id := deriveKeysetId(list_of_keys)

		for i, _ := range list_of_keys {
			list_of_keys[i].Id = id
		}

		newSeed := Seed{
			Seed:      seed,
			Active:    true,
			CreatedAt: formattedTime,
            Unit:     "sats",
            Id:       id,
		}

		saveNewSeed(conn, &newSeed)

		saveNewKeysets(conn, list_of_keys)

		if err != nil {
			log.Fatal("Error creating master key ", err)
		}

	}

	r := gin.Default()

	r.GET("/v1/keys", func(c *gin.Context) {

	    keysets := checkForActiveKeyset(conn)
		keys := orderKeysetByUnit(keysets)

		c.JSON(200, keys)

	})
    r.GET("/v1/keys/:id", func(c *gin.Context) {

        id := c.Param("id")
	    keysets := checkForKeysetById(conn, id)
		keys := orderKeysetByUnit(keysets)

		c.JSON(200, keys)

	})

    type BasicKeysetResponse struct {
        Id   string            `json:"id"`
        Unit string            `json:"unit"`
        Active bool            `json:"active"`
    }

    r.GET("/v1/keysets", func(c *gin.Context) {


	    seeds := getAllSeeds(conn)
        fmt.Println("Seeds",seeds)

        keys := make(map[string][]BasicKeysetResponse)
        keys["keysets"] = []BasicKeysetResponse{}

        for _, seed := range seeds {
            keys["keysets"] = append(keys["keysets"], BasicKeysetResponse{Id: seed.Id, Unit: seed.Unit, Active: seed.Active})

        }

		c.JSON(200, keys)

	})

    r.GET("/v1/info", func(c *gin.Context) {
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
            Name: name,
            Version: "AwesomeGoMint/0.1",
            Description: description,
            DescriptionLong: description_long,
            Motd: motd,
            Contact: contacts,
            Nuts: nuts,
        }



		c.JSON(200, response)


    })

	r.Run(":8080")
}
