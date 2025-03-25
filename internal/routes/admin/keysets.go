package admin

import (
	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/utils"
	"log/slog"
	"strconv"
)

func KeysetsPage(mint *m.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {

		c.HTML(200, "keysets.html", nil)
	}
}
func KeysetsLayoutPage(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {

	return func(c *gin.Context) {
		type KeysetData struct {
			Id        string
			Active    bool
			Unit      string
			Fees      uint
			CreatedAt int64
			Version   int
		}

		keysetArr := struct {
			Keysets []KeysetData
		}{}

		keysets, err := mint.Signer.GetKeysets()
		if err != nil {
			logger.Error("mint.Signer.GetKeysets() %+v", slog.String(utils.LogExtraInfo, err.Error()))
			c.JSON(500, "Server side error")
			return
		}

		for _, seed := range keysets.Keysets {
			keysetArr.Keysets = append(keysetArr.Keysets, KeysetData{
				Id:        seed.Id,
				Active:    seed.Active,
				Unit:      seed.Unit,
				Fees:      seed.InputFeePpk,
				// CreatedAt: seed.CreatedAt,
				// Version:   seed.Version,
			})
		}

		c.HTML(200, "keysets", keysetArr)
	}
}

type RotateRequest struct {
	Fee uint
}

func RotateSatsSeed(mint *m.Mint, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		var rotateRequest RotateRequest
		if c.ContentType() == gin.MIMEJSON {
			err := c.BindJSON(rotateRequest)
			if err != nil {
				c.JSON(400, nil)
				return
			}
		} else {
			// get Inputed fee
			feeString := c.Request.PostFormValue("FEE")

			newSeedFee, err := strconv.ParseUint(feeString, 10, 64)
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
			rotateRequest.Fee = uint(newSeedFee)
		}

		err := mint.Signer.RotateKeyset(cashu.Sat, rotateRequest.Fee)

		if err != nil {
			logger.Error(
				"mint.Signer.RotateKeyset(cashu.Sat, rotateRequest.Fee)",
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
