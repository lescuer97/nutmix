package main

import (
	"context"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	"log"
	"os"
)

var (
	DOCKER_ENV           = "DOCKER"
	MODE_ENV             = "MODE"
	MINT_PRIVATE_KEY_ENV = "MINT_PRIVATE_KEY"
)

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatal("ERROR: no .env file found and not running in docker")
	}
	ctx := context.Background()
	ctx = context.WithValue(ctx, DOCKER_ENV, os.Getenv(DOCKER_ENV))
	ctx = context.WithValue(ctx, MODE_ENV, os.Getenv(MODE_ENV))
	ctx = context.WithValue(ctx, database.DATABASE_URL_ENV, os.Getenv(database.DATABASE_URL_ENV))
	ctx = context.WithValue(ctx, mint.NETWORK_ENV, os.Getenv(mint.NETWORK_ENV))
	ctx = context.WithValue(ctx, mint.MINT_LIGHTNING_BACKEND_ENV, os.Getenv(mint.MINT_LIGHTNING_BACKEND_ENV))
	ctx = context.WithValue(ctx, comms.LND_HOST, os.Getenv(comms.LND_HOST))
	ctx = context.WithValue(ctx, comms.LND_TLS_CERT, os.Getenv(comms.LND_TLS_CERT))
	ctx = context.WithValue(ctx, comms.LND_MACAROON, os.Getenv(comms.LND_MACAROON))
	ctx = context.WithValue(ctx, comms.MINT_LNBITS_KEY, os.Getenv(comms.MINT_LNBITS_KEY))
	ctx = context.WithValue(ctx, comms.MINT_LNBITS_ENDPOINT, os.Getenv(comms.MINT_LNBITS_ENDPOINT))
	ctx = context.WithValue(ctx, "ADMIN_NOSTR_NPUB", os.Getenv("ADMIN_NOSTR_NPUB"))

	if ctx.Value(DOCKER_ENV) == "prod" {
		log.Println("Running in docker")
	}

	if ctx.Value(MODE_ENV) == "prod" {
		gin.SetMode(gin.ReleaseMode)
	}

	pool, err := database.DatabaseSetup(ctx, "migrations")

	if err != nil {
		log.Fatal("Error conecting to db", err)
	}

	seeds, err := database.GetAllSeeds(pool)

	if err != nil {
		log.Fatalf("Could not GetAllSeeds: %v", err)
	}

	mint_privkey := os.Getenv(MINT_PRIVATE_KEY_ENV)
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

	// check for seeds that are not encrypted and encrypt them
	for i, seed := range seeds {
		if !seed.Encrypted {

			err = seed.EncryptSeed(mint_privkey)

			if err != nil {
				log.Fatalf("ERROR: Could not encrypt seed that was not encrypted %+v", err)
			}

			seed.Encrypted = true

			err = database.UpdateSeed(pool, seed)
			if err != nil {
				log.Fatalf("ERROR: Could not update seeds %+v", err)
			}
			seeds[i] = seed
		}
	}

	config, err := mint.SetUpConfigFile()
	if err != nil {
		log.Fatalf("mint.SetUpConfigFile(): %+v ", err)
	}

	// remove mint private key from variable
	mint, err := mint.SetUpMint(ctx, mint_privkey, seeds, config)

	// clear mint seeds and privatekey
	seeds = []cashu.Seed{}
	mint_privkey = ""

	if err != nil {
		log.Fatalf("SetUpMint: %+v ", err)
	}

	r := gin.Default()

	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	routes.V1Routes(ctx, r, pool, mint)

	admin.AdminRoutes(ctx, r, pool, mint)

	defer pool.Close()

	r.Run(":8080")
}
