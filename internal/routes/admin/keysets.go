package admin

import (
	"encoding/hex"
	"log/slog"
	"os"
	"strconv"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/database"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
)

func KeysetsPage(pool *pgxpool.Pool, mint *m.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {

		c.HTML(200, "keysets.html", nil)
	}
}
func KeysetsLayoutPage(pool *pgxpool.Pool, mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

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
			logger.Error("database.GetAllSeeds(pool) %+v", slog.String(utils.LogExtraInfo, err.Error()))
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

func RotateSatsSeed(pool *pgxpool.Pool, mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {

		seeds, err := database.GetSeedsByUnit(pool, cashu.Sat)
		if err != nil {
			logger.Error(
				"database.GetSeedsByUnit(pool, cashu.Sat)",
				slog.String(utils.LogExtraInfo, err.Error()))
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
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "Fee was not an integer",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		// get current highest seed version
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
				slog.String(utils.LogExtraInfo, "private key is not available"))

			errorMessage := ErrorNotif{
				Error: "There was a problem getting the mint private key",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}
		decodedPrivKey, err := hex.DecodeString(mint_privkey)
		if err != nil {
			logger.Error(
				"could not parse mint private key",
				slog.String(utils.LogExtraInfo, "hex.DecodeString(mint_privkey)"))

			errorMessage := ErrorNotif{
				Error: "There was a problem getting the mint private key",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

		// rotate one level up
		generatedSeed, err := cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, highestSeed.Version+1, cashu.Sat)
		generatedSeed.Active = true

		if err != nil {
			logger.Warn(
				"There was a problem rotating the key",
				slog.String(utils.LogExtraInfo, err.Error()))

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
				slog.String(utils.LogExtraInfo, err.Error()))
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
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "there was a problem modifying the seeds",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		seeds = append(seeds, generatedSeed)

		keysets, activeKeysets, err := m.DeriveKeysetFromSeeds(seeds, parsedPrivateKey)
		if err != nil {
			logger.Error(
				"There was a problem deriving the keyset",
				slog.String(utils.LogExtraInfo, err.Error()))
			errorMessage := ErrorNotif{
				Error: "There was a problem deriving the keyset",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		mint.Keysets = keysets
		mint.ActiveKeysets = activeKeysets

		mint_privkey = ""
		parsedPrivateKey = nil
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
