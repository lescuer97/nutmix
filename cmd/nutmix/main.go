package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/lescuer97/nutmix/internal/database/postgresql"
	"github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes"
	"github.com/lescuer97/nutmix/internal/routes/admin"
	localsigner "github.com/lescuer97/nutmix/internal/signer/local_signer"
	"github.com/lescuer97/nutmix/internal/utils"
)

var (
	DOCKER_ENV           = "DOCKER"
	MODE_ENV             = "MODE"
	MINT_PRIVATE_KEY_ENV = "MINT_PRIVATE_KEY"
)

// responseWriter is a custom response writer that captures the response body
// shameless copy from https://stackoverflow.com/questions/61337843/how-to-read-the-post-request-body-in-a-gin-middleware-without-breaking-the-app
type responseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

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

	config, err := mint.SetUpConfigDB(db)
	if err != nil {
		log.Fatalf("mint.SetUpConfigDB(db): %+v ", err)
	}

	signer, err := localsigner.SetupLocalSigner(db)
	if err != nil {
		log.Fatalf("localsigner.SetupLocalSigner(db): %+v ", err)
	}

	// remove mint private key from variable
	mint, err := mint.SetUpMint(ctx, config, db, &signer)

	if err != nil {
		logger.Warn(fmt.Sprintf("SetUpMint: %+v ", err))
		return
	}
	if config.MINT_REQUIRE_AUTH {
		oidcClient, err := oidc.NewProvider(ctx, config.MINT_AUTH_OICD_URL)
		if err != nil {
			logger.Warn(fmt.Sprintf("oidc.NewProvider(ctx, config.MINT_AUTH_OICD_URL): %+v ", err))
			return
		}
		mint.OICDClient = oidcClient
	}

	r := gin.Default()

	r.Use(gin.LoggerWithWriter(w))
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowOrigins = []string{"https://" + os.Getenv("MINT_HOSTNAME"), "http://" + os.Getenv("MINT_HOSTNAME")}

	r.Use(cors.Default())

	store := persistence.NewInMemoryStore(45 * time.Minute)
	cachedPaths := map[string]bool{
		"/v1/bolt11/mint": true,
		"/v1/bolt11/melt": true,
		"/v1/swap":        true,
	}

	r.Use(func(c *gin.Context) {
		if !cachedPaths[c.Request.URL.Path] {
			c.Next()
			return
		}

		// Read the request body
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.Next()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

		// Create a cache key that includes the request body
		hash := sha256.Sum256(body)
		cacheKey := c.Request.URL.Path + "-" + fmt.Sprintf("%x", hash)

		// Check if the response is in the cache
		var cachedResponse []byte
		if err := store.Get(cacheKey, &cachedResponse); err == nil {
			c.Data(http.StatusOK, "application/json; charset=utf-8", cachedResponse)
			c.Abort()
			return
		}

		// a custom response writer to capture the response
		w := &responseWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w

		c.Next()

		// Cache the response only if the status is 200
		if c.Writer.Status() == http.StatusOK {
			store.Set(cacheKey, w.body.Bytes(), 45*time.Minute)
		}
	})

	err = mint.CheckPendingQuoteAndProofs(logger)
	if err != nil {
		logger.Error(fmt.Sprintf("SetUpMint: %+v ", err))
		return
	}
	routes.V1Routes(r, mint, logger)

	admin.AdminRoutes(ctx, r, mint, logger)

	PORT := fmt.Sprintf(":%v", 8081)

	logger.Info(fmt.Sprintf("Nutmix started in port %v", 8081))

	r.Run(PORT)
}
