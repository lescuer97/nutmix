package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/cashu"
	"log"
	"os"
)

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
	}

	databaseConUrl := os.Getenv("DATABASE_URL")

	pool, err := pgxpool.New(context.Background(), databaseConUrl)

	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

	// defer conn.Close(context.Background())

	seeds, err := GetAllSeeds(pool)

	if err != nil {
		log.Fatalf("Could not keysets: %v", err)
	}

	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {
		seed, list_of_keys, err := cashu.SetUpSeedAndKeyset()

		id, err := cashu.DeriveKeysetId(list_of_keys)

		if err != nil {
			log.Fatalf("Error DeriveKeysetId: %+v ", err)
		}

		for i, _ := range list_of_keys {
			list_of_keys[i].Id = id
		}

		err = SaveNewSeed(pool, &seed)

		seeds = append(seeds, seed)

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

	V1Routes(r, pool, mint)

    defer pool.Close()

	r.Run(":8080")
}
