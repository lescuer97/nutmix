package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
)

func main() {
	docker := os.Getenv("DOCKER")

	switch {
	case docker == "true":
		log.Println("Running in docker")
	default:
		err := godotenv.Load(".env")
		if err != nil {
			log.Fatal("ERROR: no .env file found and not running in docker")
		}
	}

	pool, err := DatabaseSetup("migrations")

	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

	seeds, err := GetAllSeeds(pool)

	if err != nil {
		log.Fatalf("Could not GetAllSeeds: %v", err)
	}

	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {
		mint_privkey := os.Getenv("MINT_PRIVATE_KEY")

		if mint_privkey == "" {
			log.Fatalf("No mint private key found in env")
		}

		generatedSeeds, err := cashu.DeriveSeedsFromKey(mint_privkey, 1, cashu.AvailableSeeds)

		err = SaveNewSeeds(pool, generatedSeeds)

		seeds = append(seeds, generatedSeeds...)

		if err != nil {
			log.Fatalf("SaveNewSeed: %+v ", err)
		}

	}

	mint, err := SetUpMint(seeds)

	if err != nil {
		log.Fatalf("SetUpMint: %+v ", err)
	}

	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	V1Routes(r, pool, mint)

	defer pool.Close()

	r.Run(":8080")
}
