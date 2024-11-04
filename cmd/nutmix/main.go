package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
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
const ConfigDirName string = "nutmix"

func main() {

	logsdir, err := utils.GetLogsDirectory()

	if err != nil {
		log.Panicln("Could not get Logs directory")
	}

	err = utils.CreateDirectoryAndPath(logsdir, mint.LogFileName)

	if err != nil {
		log.Panicf("utils.CreateDirectoryAndPath(pathToProjectDir, logFileName ) %+v", err)
	}

	pathToConfigFile := logsdir + "/" + mint.LogFileName

	// Manipulate Config file
	logFile, err := os.OpenFile(pathToConfigFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0764)
	defer logFile.Close()
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

	// check in ADMIN_NOSTR_NPUB is not empty
	if os.Getenv("ADMIN_NOSTR_NPUB") == "" {
		logger.Error("Please setup the ADMIN_NOSTR_NPUB so you can setup your mint")
		log.Panicln("Please setup the ADMIN_NOSTR_NPUB so you can setup your mint")
	}

	ctx := context.Background()

	if os.Getenv(DOCKER_ENV) == "true" {
		logger.Info("Running in docker")
	}

	if os.Getenv(MODE_ENV) == "prod" {
		gin.SetMode(gin.ReleaseMode)
		logger.Info("Running in Release mode")
	}

	db, err := postgresql.DatabaseSetup(ctx, "migrations")
	defer db.Close()

	if err != nil {
		logger.Error(fmt.Sprintf("Error conecting to db %+v", err))
		log.Panic()
	}

	seeds, err := db.GetAllSeeds()

	if err != nil {
		logger.Error(fmt.Sprintf("Could not GetAllSeeds: %v", err))
		log.Panic()
	}

	mint_privkey := os.Getenv(MINT_PRIVATE_KEY_ENV)
	if mint_privkey == "" {
		logger.Error("No mint private key found in env")
		log.Panic()
	}

	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
		logger.Error("Could not parse private key hex")
		log.Panic()
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)
	masterKey, err := mint.MintPrivateKeyToBip32(parsedPrivateKey)
	if err != nil {
		logger.Error(fmt.Sprintf("mint.MintPrivateKeyToBip32(parsedPrivateKey). %v", err))
		log.Panic()
	}

	// incase there are no seeds in the db we create a new one
	if len(seeds) == 0 {
		newSeed, err := mint.CreateNewSeed(masterKey, 1, 0)

		if err != nil {
			logger.Error(fmt.Sprintf("mint.CreateNewSeed(masterKey,1,0 ) %+v ", err))
			log.Panic()
		}

		err = db.SaveNewSeeds([]cashu.Seed{newSeed})

		seeds = append(seeds, newSeed)

		if err != nil {
			logger.Error(fmt.Sprintf("database.SaveNewSeeds(pool, []cashu.Seed{newSeed}): %+v ", err))
			log.Panic()
		}
	}

	config, err := mint.SetUpConfigFile()
	if err != nil {
		log.Fatalf("mint.SetUpConfigFile(): %+v ", err)
	}

	// remove mint private key from variable
	mint, err := mint.SetUpMint(ctx, parsedPrivateKey, seeds, config, db)

	// clear mint seeds and privatekey
	seeds = []cashu.Seed{}
	mint_privkey = ""
	parsedPrivateKey = nil
	masterKey = nil

	if err != nil {
		logger.Warn(fmt.Sprintf("SetUpMint: %+v ", err))
		return
	}

	r := gin.Default()

	r.Use(gin.LoggerWithWriter(w))
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	routes.V1Routes(r, mint, logger)

	admin.AdminRoutes(ctx, r, mint, logger)

	logger.Info("Nutmix started in port 8080")

	r.Run(":8080")
}
