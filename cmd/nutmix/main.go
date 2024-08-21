package main

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	"github.com/lescuer97/nutmix/internal/utils"
	"io"
	"log"
	"log/slog"
	"os"
)

var (
	DOCKER_ENV           = "DOCKER"
	MODE_ENV             = "MODE"
	MINT_PRIVATE_KEY_ENV = "MINT_PRIVATE_KEY"
)

const ConfigFileName string = "config.toml"
const ConfigDirName string = ".nutmix"
const logFileName string = "nutmix.log"

func main() {

	dir, err := os.UserHomeDir()

	if err != nil {
		log.Panicln("Could not get Home directory")
	}
	var pathToProjectDir string = dir + "/" + ConfigDirName

	if os.Getenv(DOCKER_ENV) == "true" {
		pathToProjectDir = "/var/log/nutmix"
	}

	err = utils.CreateDirectoryAndPath(pathToProjectDir, logFileName)

	if err != nil {
		log.Panicf("utils.CreateDirectoryAndPath(pathToProjectDir, logFileName ) %+v", err)
	}

	pathToConfigFile := pathToProjectDir + "/" + logFileName

	// Manipulate Config file
	logFile, err := os.OpenFile(pathToConfigFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0764)
	if err != nil {
		log.Panicf("os.OpenFile(pathToProjectLogFile, os.O_RDWR|os.O_CREATE, 0764) %+v", err)
	}

	w := io.MultiWriter(os.Stdout, logFile)

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	logger := slog.New(slog.NewJSONHandler(w, opts))

	err = godotenv.Load(".env")

	if err != nil {
		logger.Error("ERROR: no .env file found and not running in docker")
		log.Panic()
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

	if ctx.Value(DOCKER_ENV) == "true" {
		logger.Info("Running in docker")
	}

	if ctx.Value(MODE_ENV) == "prod" {
		gin.SetMode(gin.ReleaseMode)
		logger.Info("Running in Release mode")
	}

	pool, err := database.DatabaseSetup(ctx, "migrations")

	if err != nil {
		logger.Error(fmt.Sprintf("Error conecting to db %+v", err))
		log.Panic()
	}

	seeds, err := database.GetAllSeeds(pool)

	if err != nil {
		logger.Error(fmt.Sprintf("Could not GetAllSeeds: %v", err))
		log.Panic()
	}

	mint_privkey := os.Getenv(MINT_PRIVATE_KEY_ENV)
	if mint_privkey == "" {
		logger.Error("No mint private key found in env")
		log.Panic()
	}

	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {

		generatedSeeds, err := cashu.DeriveSeedsFromKey(mint_privkey, 1, cashu.AvailableSeeds)

		if err != nil {
			logger.Error(fmt.Sprintf("ERROR: DeriveSeedsFromKey: %+v ", err))
			log.Panic()
		}

		err = database.SaveNewSeeds(pool, generatedSeeds)

		seeds = append(seeds, generatedSeeds...)

		if err != nil {
			logger.Error(fmt.Sprintf("SaveNewSeed: %+v ", err))
			log.Panic()
		}
	}

	inactiveUnits, err := mint.CheckForInactiveSeeds(seeds)

	if err != nil {
		logger.Error(fmt.Sprintf("ERROR: CheckForActiveSeeds: %+v ", err))
		log.Panic()
	}

	// if there are inactive seeds we derive new seeds from the mint private key and version up
	if len(inactiveUnits) > 0 {
		logger.Info(fmt.Sprintf("Deriving new seeds for activation: %+v", inactiveUnits))

		var versionedUpSeeds []cashu.Seed
		for _, seedType := range inactiveUnits {

			generatedSeed, err := cashu.DeriveIndividualSeedFromKey(mint_privkey, seedType.Version+1, seedType.Unit)

			if err != nil {
				logger.Warn(fmt.Sprintf(" cashu.DeriveIndividualSeedFromKey INCREASE Version: %+v ", err))
				log.Panic()
			}

			versionedUpSeeds = append(versionedUpSeeds, generatedSeed)
		}

		err = database.SaveNewSeeds(pool, versionedUpSeeds)
		if err != nil {
			logger.Warn(fmt.Sprintf("SaveNewSeed: %+v ", err))
			log.Panic()
		}

		seeds = append(seeds, versionedUpSeeds...)
	}

	// check for seeds that are not encrypted and encrypt them
	for i, seed := range seeds {
		if !seed.Encrypted {

			err = seed.EncryptSeed(mint_privkey)

			if err != nil {
				logger.Error(fmt.Sprintf("Could not encrypt seed that was not encrypted %+v", err))
				log.Panic()
			}

			seed.Encrypted = true

			err = database.UpdateSeed(pool, seed)
			if err != nil {
				logger.Error(fmt.Sprintf("Could not update seeds %+v", err))
				log.Panic()
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
		logger.Warn(fmt.Sprintf("SetUpMint: %+v ", err))
		return
	}

	r := gin.Default()

	r.Use(gin.LoggerWithWriter(w))
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	routes.V1Routes(ctx, r, pool, mint, logger)

	admin.AdminRoutes(ctx, r, pool, mint)

	defer pool.Close()

	logger.Info("Nutmix started in port 8080")

	r.Run(":8080")
}
