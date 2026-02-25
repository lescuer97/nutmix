package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	"github.com/lescuer97/nutmix/internal/routes/middleware"
	"github.com/lescuer97/nutmix/internal/signer"
	"github.com/lescuer97/nutmix/internal/stats"
	"github.com/lightningnetwork/lnd/zpay32"

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

	err = utils.CreateDirectoryAndPath(logsdir, utils.LogFileName)
	if err != nil {
		log.Panicf("utils.CreateDirectoryAndPath(pathToProjectDir, logFileName ) %+v", err)
	}

	pathToConfigFile := logsdir + "/" + utils.LogFileName

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

	//nolint:exhaustruct
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

	baseJSONHandler := slog.NewJSONHandler(w, opts)

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer startupCancel()
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgresql.DatabaseSetup(startupCtx, "migrations")
	if err != nil {
		slog.Error("Error conecting to db", slog.Any("error", err))
		log.Panic()
	}
	defer db.Close()

	config, nostrNotificationConfig, err := mint.SetUpConfigDB(startupCtx, db)
	if err != nil {
		log.Fatalf("mint.SetUpConfigDB(ctx, db): %+v ", err)
	}

	signer, err := GetSignerFromValue(os.Getenv("SIGNER_TYPE"), db)
	if err != nil {
		log.Fatalf("signer.GetSignerFromValue(os.Getenv(), db): %+v ", err)
	}

	// remove mint private key from variable
	mint, err := mint.SetUpMint(startupCtx, config, nostrNotificationConfig, db, signer)

	if err != nil {
		slog.Warn("SetUpMint", slog.Any("error", err))
		return
	}

	logger := slog.New(admin.NewNostrErrorNotifyHandler(baseJSONHandler, mint))
	slog.SetDefault(logger)

	r := gin.Default()

	r.Use(gin.LoggerWithWriter(w))

	r.Use(cors.Default())
	// // gzip compression
	// r.Use(gzip.Gzip(gzip.DefaultCompression))

	store := persistence.NewInMemoryStore(45 * time.Minute)

	r.Use(middleware.CacheMiddleware(store))

	// Add per-request timeout middleware (sets context deadline for handlers)
	r.Use(middleware.TimeoutMiddleware(90 * time.Second))

	err = mint.CheckPendingQuoteAndProofs()
	if err != nil {
		slog.Error("SetUpMint", slog.Any("error", err))
		return
	}

	statsService := stats.Service{
		DB:        db,
		Now:       time.Now,
		Logger:    nil,
		NewTicker: nil,
		DecodeMintAmount: func(request string) (uint64, error) {
			invoice, err := zpay32.Decode(request, mint.LightningBackend.GetNetwork())
			if err != nil {
				return 0, err
			}
			if invoice.MilliSat == nil {
				return 0, fmt.Errorf("invoice has no amount")
			}
			return uint64(invoice.MilliSat.ToSatoshis()), nil
		},
	}
	go statsService.Run(appCtx, 15*time.Minute)

	routes.V1Routes(r, mint)

	admin.AdminRoutes(appCtx, r, mint)

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
	signalCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	// Define a custom http.Server
	//nolint:exhaustruct
	srv := &http.Server{
		Addr:         PORT,
		Handler:      r,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 4 * time.Second,
		IdleTimeout:  3 * time.Minute,
	}

	var shutdownOnce sync.Once
	shutdown := func() {
		shutdownOnce.Do(func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			if err := shutdownServerAndBackend(shutdownCtx, srv, mint); err != nil {
				slog.Warn("shutdown finished with errors", slog.Any("error", err))
			}
		})
	}

	go func() {
		<-signalCtx.Done()
		slog.Info("shutdown signal received")
		shutdown()
	}()

	// Start the server
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", slog.Any("error", err))
		os.Exit(1)
	}

	shutdown()
}

func shutdownServerAndBackend(ctx context.Context, srv *http.Server, mintInstance *mint.Mint) error {
	var shutdownErr error

	if srv != nil {
		slog.Info("shutting down http server")
		if err := srv.Shutdown(ctx); err != nil && err != http.ErrServerClosed {
			shutdownErr = err
		}
	}

	if err := stopLDKBackend(mintInstance); err != nil {
		if shutdownErr != nil {
			shutdownErr = fmt.Errorf("http shutdown: %w; ldk shutdown: %w", shutdownErr, err)
		} else {
			shutdownErr = err
		}
		return shutdownErr
	}

	slog.Info("shutdown complete")
	return shutdownErr
}

func stopLDKBackend(mintInstance *mint.Mint) error {
	if mintInstance == nil {
		return nil
	}

	ldkBackend, ok := mintInstance.LightningBackend.(*ldk.LDK)
	if !ok {
		return nil
	}

	slog.Info("stopping ldk backend")
	if err := ldkBackend.Stop(); err != nil {
		return fmt.Errorf("failed to stop ldk backend: %w", err)
	}

	return nil
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
