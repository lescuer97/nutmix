package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
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
	mode := os.Getenv("MODE")

	if mode == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	pool, err := database.DatabaseSetup("migrations")

	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

	seeds, err := database.GetAllSeeds(pool)

	if err != nil {
		log.Fatalf("Could not GetAllSeeds: %v", err)
	}

	mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
	if mint_privkey == "" {
		log.Fatalf("No mint private key found in env")
	}

	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {

		generatedSeeds, err := cashu.DeriveSeedsFromKey(mint_privkey, 1, cashu.AvailableSeeds)

		if err != nil {
			log.Fatalf("ERROR: DeriveSeedsFromKey: %+v ", err)
		}

		err = database.SaveNewSeeds(pool, generatedSeeds)

		seeds = append(seeds, generatedSeeds...)

		if err != nil {
			log.Fatalf("SaveNewSeed: %+v ", err)
		}
	}

	inactiveUnits, err := mint.CheckForInactiveSeeds(seeds)

	if err != nil {
		log.Fatalf("ERROR: CheckForActiveSeeds: %+v ", err)
	}

	log.Printf("INFO: Inactive units: %+v", inactiveUnits)

	// if there are inactive seeds we derive new seeds from the mint private key and version up
	if len(inactiveUnits) > 0 {
		log.Printf("INFO: Deriving new seeds for activation: %+v", inactiveUnits)

		var versionedUpSeeds []cashu.Seed
		for _, seedType := range inactiveUnits {

			generatedSeed, err := cashu.DeriveIndividualSeedFromKey(mint_privkey, seedType.Version+1, seedType.Unit)

			if err != nil {
				log.Fatalf("ERROR: cashu.DeriveIndividualSeedFromKey INCREASE Version: %+v ", err)
			}

			versionedUpSeeds = append(versionedUpSeeds, generatedSeed)
		}

		err = database.SaveNewSeeds(pool, versionedUpSeeds)
		if err != nil {
			log.Fatalf("SaveNewSeed: %+v ", err)
		}

		seeds = append(seeds, versionedUpSeeds...)

	}

	mint, err := mint.SetUpMint(seeds)

	if err != nil {
		log.Fatalf("SetUpMint: %+v ", err)
	}

	r := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	routes.V1Routes(r, pool, mint)

	defer pool.Close()

	r.Run(":8080")
}
