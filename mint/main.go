package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/cashu"
	"github.com/tyler-smith/go-bip32"
	"log"
	"os"
	"time"
)

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

	seeds, err := GetAllSeeds(conn)

	if err != nil {
		log.Fatalf("Could not keysets: %v", err)
	}

	if len(seeds) == 0 {
		seed, err := bip32.NewSeed()

		if err != nil {
			log.Fatalf("Error creating seed: %+v ", err)
		}
		// Get the current time
		currentTime := time.Now().Unix()

		// // Format the time as a string
		masterKey, err := bip32.NewMasterKey(seed)

		list_of_keys := cashu.GenerateKeysets(masterKey, cashu.PosibleKeysetValues, "")

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
			Unit:      cashu.Sat.String(),
			Id:        id,
		}

		err = SaveNewSeed(conn, &newSeed)

		seeds = append(seeds, newSeed)

		if err != nil {
			log.Fatalf("SaveNewSeed: %+v ", err)
		}

	}

	mint, err := SetUpMint(seeds)

	if err != nil {
		log.Fatalf("SetUpMint: %+v ", err)
	}

	r := gin.Default()

	r.Use(cors.Default())

	V1Routes(r, conn, mint)

	r.Run(":8080")
}
