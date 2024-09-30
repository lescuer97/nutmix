package admin

import (
	"encoding/hex"
	"fmt"
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

type RotateRequest struct {
	Fee int
}

func rotateSatsSeed(pool *pgxpool.Pool, mint *m.Mint, rotateRequest RotateRequest) error {
	seeds, err := database.GetSeedsByUnit(pool, cashu.Sat)
	if err != nil {

		return fmt.Errorf("database.GetSeedsByUnit(pool, cashu.Sat). %w", err)
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
		return fmt.Errorf(`os.Getenv("MINT_PRIVATE_KEY"). %w`, err)
	}
	decodedPrivKey, err := hex.DecodeString(mint_privkey)
	if err != nil {
		return fmt.Errorf(`hex.DecodeString(mint_privkey). %w`, err)
	}

	parsedPrivateKey := secp256k1.PrivKeyFromBytes(decodedPrivKey)

	// rotate one level up
	generatedSeed, err := cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, highestSeed.Version+1, cashu.Sat)
	generatedSeed.Active = true

	if err != nil {
		return fmt.Errorf(`cashu.DeriveIndividualSeedFromKey(parsedPrivateKey, highestSeed.Version+1, cashu.Sat). %w`, err)
	}

	generatedSeed.InputFeePpk = rotateRequest.Fee

	// add new key to db
	err = database.SaveNewSeed(pool, &generatedSeed)
	if err != nil {
		return fmt.Errorf(`database.SaveNewSeed(pool, &generatedSeed). %w`, err)
	}
	err = database.UpdateActiveStatusSeeds(pool, seeds)
	if err != nil {
		return fmt.Errorf(`database.UpdateActiveStatusSeeds(pool, seeds). %w`, err)
	}

	seeds = append(seeds, generatedSeed)

	keysets, activeKeysets, err := m.DeriveKeysetFromSeeds(seeds, parsedPrivateKey)
	if err != nil {
		return fmt.Errorf(`m.DeriveKeysetFromSeeds(seeds, parsedPrivateKey). %w`, err)
	}

	fmt.Printf("mint.ActiveKeysets: %+v", mint.ActiveKeysets)
	mint.Keysets = keysets
	mint.ActiveKeysets = activeKeysets
	fmt.Printf("ActiveKeysets: %+v", activeKeysets)

	mint_privkey = ""
	parsedPrivateKey = nil
	return nil
}

func RotateSatsSeed(pool *pgxpool.Pool, mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rotateRequest RotateRequest

		fmt.Printf("ROTATE FUNCTION")
		if c.ContentType() == gin.MIMEJSON {
			err := c.BindJSON(rotateRequest)
			if err != nil {
				c.JSON(400, nil)
				return
			}
		} else {
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
			rotateRequest.Fee = newSeedFee
		}

		fmt.Printf("ROTATE FUNCTION")
		err := rotateSatsSeed(pool, mint, rotateRequest)

		if err != nil {
			logger.Error(
				"otateSatsSeed(pool,mint, logger, rotateRequest)",
				slog.String(utils.LogExtraInfo, err.Error()))

			errorMessage := ErrorNotif{
				Error: "There was an error getting the seeds",
			}

			c.HTML(200, "settings-error", errorMessage)
			return
		}

		if c.ContentType() == gin.MIMEJSON {
			c.JSON(200, nil)
		} else {

			successMessage := struct {
				Success string
			}{
				Success: "Key succesfully rotated",
			}
			c.Header("HX-Trigger", "recharge-keyset")
			c.HTML(200, "settings-success", successMessage)
		}
	}
}
