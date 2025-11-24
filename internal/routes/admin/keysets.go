package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
	"github.com/lescuer97/nutmix/internal/utils"
)

var ErrUnitNotCorrect = errors.New("Unit not correct")
var ErrNoExpiryTime = errors.New("No expiry time provided")

func KeysetsPage(mint *m.Mint) gin.HandlerFunc {

	return func(c *gin.Context) {
		ctx := context.Background()
		err := templates.KeysetsPage().Render(ctx, c.Writer)

		if err != nil {
			c.Error(fmt.Errorf("templates.KeysetsPage().Render(ctx, c.Writer). %w", err))
			// c.HTML(400,"", nil)
			return
		}

	}
}
func KeysetsLayoutPage(adminHandler *adminHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		keysetMap, err := adminHandler.getKeysets(nil)
		ctx := context.Background()
		err = templates.KeysetsList(keysetMap).Render(ctx, c.Writer)

		if err != nil {
			c.Error(fmt.Errorf("templates.KeysetsList(keysetArr.Keysets).Render(ctx, c.Writer). %w", err))
			return
		}
	}
}

type RotateRequest struct {
	Fee              uint
	Unit             cashu.Unit
	ExpireLimitHours uint
}

func RotateSatsSeed(adminHandler *adminHandler) gin.HandlerFunc {
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

			unitStr := c.Request.PostFormValue("UNIT")

			if unitStr == "" {
				c.Error(ErrUnitNotCorrect)
				return
			}

			expireLimitStr := c.Request.PostFormValue("EXPIRE_LIMIT")
			if expireLimitStr == "" {
				c.Error(ErrNoExpiryTime)
				return
			}

			unit, err := cashu.UnitFromString(unitStr)
			if err != nil {
				c.Error(fmt.Errorf("cashu.UnitFromString(unitStr). %w. %w", err, ErrUnitNotCorrect))
				return
			}
			rotateRequest.Unit = unit

			newSeedFee, err := strconv.ParseUint(feeString, 10, 64)
			if err != nil {
				slog.Error(
					"Err: There was a problem rotating the key",
					slog.String(utils.LogExtraInfo, err.Error()))

				errorMessage := ErrorNotif{
					Error: "Fee was not an integer",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}
			rotateRequest.Fee = uint(newSeedFee)

			expiryLimit, err := strconv.ParseUint(expireLimitStr, 10, 64)
			if err != nil {
				slog.Error(
					"Err: There was a problem rotating the key",
					slog.String(utils.LogExtraInfo, err.Error()))

				errorMessage := ErrorNotif{
					Error: "Expire limit is not an integer",
				}

				c.HTML(200, "settings-error", errorMessage)
				return
			}
			rotateRequest.ExpireLimitHours = uint(expiryLimit)
		}

		err := adminHandler.rotateKeyset(rotateRequest.Unit, rotateRequest.Fee, rotateRequest.ExpireLimitHours)
		if err != nil {
			slog.Error(
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
