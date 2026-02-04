package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	"github.com/lescuer97/nutmix/internal/routes/middleware"
	"github.com/lescuer97/nutmix/internal/signer"

	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
	remoteSigner "github.com/lescuer97/nutmix/internal/signer/remote_signer"
	"github.com/lescuer97/nutmix/internal/utils"
)

var (
	DOCKER_ENV           = "DOCKER"
	MODE_ENV             = "MODE"
	MINT_PRIVATE_KEY_ENV = "MINT_PRIVATE_KEY"
	PORT                 = "PORT"
)

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
	logFile, err := os.OpenFile(pathToConfigFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		log.Panicf("os.OpenFile(pathToProjectLogFile, os.O_RDWR|os.O_CREATE, 0764) %+v", err)
	}
	defer func() {
		if err := logFile.Close(); err != nil {
			slog.Warn("failed to close log file", slog.Any("error", err))
		}
	}()

	w := io.MultiWriter(os.Stdout, logFile)

	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	err = godotenv.Load(".env")
	if err != nil {
		log.Printf("Did not find any .env file using environment variables!")
	}

	gin.SetMode(gin.ReleaseMode)

	if os.Getenv("DEBUG") == "true" {
		gin.SetMode(gin.DebugMode)
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}

	logger := slog.New(slog.NewJSONHandler(w, opts))
	slog.SetDefault(logger)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5)*time.Minute)
	defer cancel()

	db, err := postgresql.DatabaseSetup(ctx, "migrations")
	if err != nil {
		slog.Error("Error conecting to db", slog.Any("error", err))
		log.Panic()
	}
	defer db.Close()

	config, err := mint.SetUpConfigDB(db)
	if err != nil {
		log.Fatalf("mint.SetUpConfigDB(db): %+v ", err)
	}

	signer, err := GetSignerFromValue(os.Getenv("SIGNER_TYPE"), db)
	if err != nil {
		log.Fatalf("signer.GetSignerFromValue(os.Getenv(), db): %+v ", err)
	}

	// remove mint private key from variable
	mint, err := mint.SetUpMint(ctx, config, db, signer)

	if err != nil {
		slog.Warn("SetUpMint", slog.Any("error", err))
		return
	}

	r := gin.Default()

	r.Use(gin.LoggerWithWriter(w))

	r.Use(cors.Default())
	// // gzip compression
	// r.Use(gzip.Gzip(gzip.DefaultCompression))

	store := persistence.NewInMemoryStore(45 * time.Minute)

	r.Use(middleware.CacheMiddleware(store))

	err = mint.CheckPendingQuoteAndProofs()
	if err != nil {
		slog.Error("SetUpMint", slog.Any("error", err))
		return
	}
	routes.V1Routes(r, mint)

	admin.AdminRoutes(ctx, r, mint)

	PORT = ":8081"
	PORTStr := os.Getenv("PORT")
	if PORTStr != "" {
		portInt, err := strconv.ParseUint(PORTStr, 10, 64)
		if err != nil {
			slog.Error("Your picked port is not correct", slog.Any("error", err))
			return
		}
		PORT = fmt.Sprintf(":%v", portInt)
	}

	slog.Info("Nutmix started in port", slog.String("port", PORT))

	// Define a custom http.Server
	srv := &http.Server{
		Addr:         PORT,
		Handler:      r,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 4 * time.Second,
		IdleTimeout:  2 * time.Minute,
	}
	// Start the server
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}
}

const MemorySigner = "memory"
const AbstractSocketSigner = "abstract_socket"
const NetworkSigner = "network"

func GetSignerFromValue(signerType string, db database.MintDB) (signer.Signer, error) {
	switch signerType {
	case MemorySigner:
		signer, err := localsigner.SetupLocalSigner(db)
		if err != nil {
			return &signer, fmt.Errorf("localsigner.SetupLocalSigner(db): %+v ", err)
		}
		return &signer, nil
	case AbstractSocketSigner:
		signer, err := remoteSigner.SetupRemoteSigner(false, os.Getenv("NETWORK_SIGNER_ADDRESS"))
		if err != nil {
			return &signer, fmt.Errorf("socketremotesigner.SetupSocketSigner(): %+v ", err)
		}
		return &signer, nil

	case NetworkSigner:
		signer, err := remoteSigner.SetupRemoteSigner(true, os.Getenv("NETWORK_SIGNER_ADDRESS"))
		if err != nil {
			return &signer, fmt.Errorf("socketremotesigner.SetupSocketSigner(): %+v ", err)
		}
		return &signer, nil

	default:
		return nil, fmt.Errorf("no signer type has been selected")
	}

}
