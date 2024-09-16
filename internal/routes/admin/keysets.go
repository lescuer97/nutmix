package admin

import (
	"log/slog"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	"github.com/lescuer97/nutmix/internal/mint"
)

func KeysetsPage(pool *pgxpool.Pool, mint *mint.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {

		c.HTML(200, "keysets.html", nil)
	}
}
func KeysetsLayoutPage(pool *pgxpool.Pool, mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {
		type KeysetData struct {
			Id        string
			Active    bool
			Unit      string
			Fees      int
			CreatedAt int64
			Version   int
		}

		keysetArr := struct {
			Keysets []KeysetData
		}{}

		seeds, err := database.GetAllSeeds(pool)
		if err != nil {
			logger.Error("database.GetAllSeeds(pool) %+v", slog.String("extra-info", err.Error()))
			c.JSON(500, "Server side error")
			return
		}

		for _, seed := range seeds {
			keysetArr.Keysets = append(keysetArr.Keysets, KeysetData{
				Id:        seed.Id,
				Active:    seed.Active,
				Unit:      seed.Unit,
				Fees:      seed.InputFeePpk,
				CreatedAt: seed.CreatedAt,
				Version:   seed.Version,
			})
		}

		c.HTML(200, "keysets", keysetArr)
	}
}

func RotateSatsSeed(pool *pgxpool.Pool, mint *mint.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		seeds, err := database.GetSeedsByUnit(pool, cashu.Sat)
		if err != nil {
			logger.Error(
				"database.GetSeedsByUnit(pool, cashu.Sat)",
				slog.String("extra-info", err.Error()))
			errorMessage := ErrorNotif{
				Error: "There was an error getting the seeds",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}
		// get Inputed fee
		feeString := c.Request.PostFormValue("FEE")

		newSeedFee, err := strconv.Atoi(feeString)

		if err != nil {
			logger.Error(
				"Err: There was a problem rotating the key",
				slog.String("extra-info", err.Error()))

			errorMessage := ErrorNotif{
				Error: "Fee was not an integer",
			}

			c.HTML(200, "settings-error", errorMessage)
			return

		}

		// get current highted seed version

		var highestSeed cashu.Seed

		for i, seed := range seeds {
			if highestSeed.Version < seed.Version {
				highestSeed = seed
			}
			seeds[i].Active = false
		}
		// get mint private_key

		mint_privkey := os.Getenv("MINT_PRIVATE_KEY")
		if mint_privkey == "" {
			logger.Error(
				"Err: could not get mint private key",
				slog.String("extra-info", err.Error()))
			errorMessage := ErrorNotif{
				Error: "There was a problem getting the mint private key",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		// rotate one level up
		generatedSeed, err := cashu.DeriveIndividualSeedFromKey(mint_privkey, highestSeed.Version+1, cashu.Sat)

		if err != nil {
			logger.Warn(
				"There was a problem rotating the key",
				slog.String("extra-info", err.Error()))

			errorMessage := ErrorNotif{
				Error: "There was a problem rotating the key",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		generatedSeed.InputFeePpk = newSeedFee

		// add new key to db
		err = database.SaveNewSeed(pool, &generatedSeed)
		if err != nil {
			logger.Error(
				"Could not save key",
				slog.String("extra-info", err.Error()))
			errorMessage := ErrorNotif{
				Error: "There was a problem saving the new seed",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}
		err = database.UpdateActiveStatusSeeds(pool, seeds)
		if err != nil {
			logger.Error(
				"database.UpdateActiveStatusSeeds",
				slog.String("extra-info", err.Error()))

			errorMessage := ErrorNotif{
				Error: "there was a problem modifying the seeds",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		seeds = append(seeds, generatedSeed)

		// regenerate keysets for mint use
		newKeysets := make(map[string][]cashu.Keyset)
		newActiveKeysets := make(map[string]cashu.KeysetMap)

		for _, seed := range seeds {
			keysets, err := seed.DeriveKeyset(mint_privkey)
			if err != nil {
				logger.Error(
					"There was a problem deriving the keyset",
					slog.String("extra-info", err.Error()))
				errorMessage := ErrorNotif{
					Error: "There was a problem deriving the keyset",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}

			if seed.Active {
				newActiveKeysets[seed.Unit] = make(cashu.KeysetMap)
				for _, keyset := range keysets {
					mint.ActiveKeysets[seed.Unit][keyset.Amount] = keyset
				}

			}

			newKeysets[seed.Unit] = append(mint.Keysets[seed.Unit], keysets...)
		}

		mint.Keysets = newKeysets
		mint.ActiveKeysets = newActiveKeysets

		mint_privkey = ""
		seeds = []cashu.Seed{}

		successMessage := struct {
			Success string
		}{
			Success: "Key succesfully rotated",
		}
		c.Header("HX-Trigger", "recharge-keyset")
		c.HTML(200, "settings-success", successMessage)
	}
}
