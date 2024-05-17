package main

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
	"log"
	"os"
)

func main() {
	docker := os.Getenv("DOCKER")

	switch {
	case docker == "true":
		log.Println("Running in docker")
	default:
		err := godotenv.Load("../.env")
		if err != nil {
			log.Fatal("ERROR: no .env file found and not running in docker")
		}
	}

	pool, err := DatabaseSetup()

	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

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

		for i := range list_of_keys {
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
