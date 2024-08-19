package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"log/syslog"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/comms"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/utils"
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

	sysLogger, err := syslog.New(syslog.LOG_WARNING|syslog.LOG_DAEMON, "nutmix")
	if err != nil {
		log.Fatalf("Could not setup syslog %+v", err)
	}

	defer sysLogger.Close()

	dir, err := os.UserHomeDir()

	if err != nil {
		log.Panicln("Could not get Home directory")
	}
	var pathToProjectDir string = dir + "/" + ConfigDirName
	err = utils.CreateDirectoryAndPath(pathToProjectDir, logFileName)

	if err != nil {
		log.Panicf("utils.CreateDirectoryAndPath(pathToProjectDir, logFileName ) %+v", err)
	}

	pathToConfigFile := pathToProjectDir + "/" + ConfigDirName

	// Manipulate Config file
	logFile, err := os.OpenFile(pathToConfigFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0764)
	if err != nil {
		log.Panicf("os.OpenFile(pathToProjectLogFile, os.O_RDWR|os.O_CREATE, 0764) %+v", err)
	}

	w := io.MultiWriter(os.Stdout, sysLogger, logFile)

	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	logger := slog.New(slog.NewJSONHandler(w, opts))

	err = godotenv.Load(".env")

	if err != nil {
		sysLogger.Alert("ERROR: no .env file found and not running in docker")
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

	if ctx.Value(DOCKER_ENV) == "prod" {
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
		sysLogger.Alert("No mint private key found in env")
	}

	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {

		generatedSeeds, err := cashu.DeriveSeedsFromKey(mint_privkey, 1, cashu.AvailableSeeds)

		if err != nil {
			sysLogger.Alert(fmt.Sprintf("ERROR: DeriveSeedsFromKey: %+v ", err))
		}

		err = database.SaveNewSeeds(pool, generatedSeeds)

		seeds = append(seeds, generatedSeeds...)

		if err != nil {
			sysLogger.Alert(fmt.Sprintf("SaveNewSeed: %+v ", err))
		}
	}

	inactiveUnits, err := mint.CheckForInactiveSeeds(seeds)

	if err != nil {
		sysLogger.Alert(fmt.Sprintf("ERROR: CheckForActiveSeeds: %+v ", err))
	}

	// if there are inactive seeds we derive new seeds from the mint private key and version up
	if len(inactiveUnits) > 0 {
		sysLogger.Info(fmt.Sprintf("Deriving new seeds for activation: %+v", inactiveUnits))

		var versionedUpSeeds []cashu.Seed
		for _, seedType := range inactiveUnits {

			generatedSeed, err := cashu.DeriveIndividualSeedFromKey(mint_privkey, seedType.Version+1, seedType.Unit)

			if err != nil {
				sysLogger.Alert(fmt.Sprintf(" cashu.DeriveIndividualSeedFromKey INCREASE Version: %+v ", err))
			}

			versionedUpSeeds = append(versionedUpSeeds, generatedSeed)
		}

		err = database.SaveNewSeeds(pool, versionedUpSeeds)
		if err != nil {
			sysLogger.Alert(fmt.Sprintf("SaveNewSeed: %+v ", err))
		}

		seeds = append(seeds, versionedUpSeeds...)
	}

	// check for seeds that are not encrypted and encrypt them
	for i, seed := range seeds {
		if !seed.Encrypted {

			err = seed.EncryptSeed(mint_privkey)

			if err != nil {
				sysLogger.Err(fmt.Sprintf("Could not encrypt seed that was not encrypted %+v", err))
			}

			seed.Encrypted = true

			err = database.UpdateSeed(pool, seed)
			if err != nil {
				sysLogger.Alert(fmt.Sprintf("Could not update seeds %+v", err))
			}
			seeds[i] = seed
		}
	}

	// remove mint private key from variable

	mint, err := mint.SetUpMint(ctx, mint_privkey, seeds)

	// clear mint seeds and privatekey
	seeds = []cashu.Seed{}
	mint_privkey = ""

	if err != nil {
		logger.Warn(fmt.Sprintf("SetUpMint: %+v ", err))
		return
	}

	r := gin.Default()

	r.Use(gin.LoggerWithWriter(w))
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	routes.V1Routes(ctx, r, pool, mint, logger)

	defer pool.Close()

	logger.Info("Nutmix started in port 8080")

	r.Run(":8080")
}
