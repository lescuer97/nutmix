package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

const lightningStatusTimeout = 5 * time.Second

func LightningDataFormFields(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		backend := strings.TrimSpace(c.Request.FormValue(m.MINT_LIGHTNING_BACKEND_ENV))
		if backend == "" {
			backend = string(mint.Config.MINT_LIGHTNING_BACKEND)
		}
		resources := getLDKResourceSnapshot()
		ldkForm := getLDKFormValues(c, mint)

		ctx := c.Request.Context()
		err := templates.SetupForms(backend, mint.Config, resources, ldkForm).Render(ctx, c.Writer)

		if err != nil {
			_ = c.Error(fmt.Errorf("templates.SetupForms(mint.Config).Render(ctx, c.Writer). %w", err))
			return
		}
	}
}

func LightningBackendStatus(mint *m.Mint) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeStatus := lightning.UNKNOWN_STATUS
		deprecated := false
		backend := mint.LightningBackend
		if backend != nil {
			ctx, cancel := context.WithTimeout(c.Request.Context(), lightningStatusTimeout)
			defer cancel()

			status, err := backend.Status(ctx)
			if errors.Is(err, lightning.ErrLNBackendEndOfLife) {
				slog.Error("lightning backend is end of life and must be replaced", slog.Any("error", err))
				nodeStatus = lightning.STOPPED_STATUS
			} else if err != nil {
				slog.Warn("could not check lightning backend status", slog.Any("error", err))
				nodeStatus = lightning.OFFLINE_STATUS
			} else {
				nodeStatus = status
			}

			switch backend.LightningType() {
			case lightning.LNBITS: //nolint:staticcheck // LNBITS remains supported until its scheduled removal.
				deprecated = true
			}
		}

		switch nodeStatus {
		case lightning.ONLINE_STATUS, lightning.OFFLINE_STATUS, lightning.UNKNOWN_STATUS, lightning.STOPPED_STATUS:
		default:
			nodeStatus = lightning.UNKNOWN_STATUS
		}

		if err := templates.LightningBackendStatus(nodeStatus, deprecated).Render(c.Request.Context(), c.Writer); err != nil {
			_ = c.Error(fmt.Errorf("templates.LightningBackendStatus(nodeStatus, deprecated).Render: %w", err))
		}
	}
}

func getLDKFormValues(c *gin.Context, mint *m.Mint) templates.LDKFormValues {
	formValues := templates.LDKFormValues{
		ChainSourceType:   string(ldk.ChainSourceBitcoind),
		Address:           "",
		Port:              "",
		Username:          "",
		Password:          "",
		ElectrumServerURL: "",
		EsploraServerURL:  "",
	}

	persistedConfig, err := ldk.GetPersistedConfig(c.Request.Context(), mint.MintDB)
	if err == nil {
		formValues.ChainSourceType = string(persistedConfig.ChainSourceType)
		formValues.Address = persistedConfig.Rpc.Address
		if persistedConfig.Rpc.Port != 0 {
			formValues.Port = strconv.FormatUint(uint64(persistedConfig.Rpc.Port), 10)
		}
		formValues.Username = persistedConfig.Rpc.Username
		formValues.ElectrumServerURL = persistedConfig.ElectrumServerURL
		formValues.EsploraServerURL = persistedConfig.EsploraServerURL
	}

	if value := requestFormValue(c, "LDK_CHAIN_SOURCE_TYPE"); value != "" {
		formValues.ChainSourceType = normalizeLDKChainSourceType(value)
	}
	if value := requestFormValue(c, "BITCOIN_NODE_RPC_ADDRESS"); value != "" {
		formValues.Address = value
	}
	if value := requestFormValue(c, "BITCOIN_NODE_RPC_PORT"); value != "" {
		formValues.Port = value
	}
	if value := requestFormValue(c, "BITCOIN_NODE_RPC_USERNAME"); value != "" {
		formValues.Username = value
	}
	if value := requestFormValue(c, "BITCOIN_NODE_RPC_PASSWORD"); value != "" {
		formValues.Password = value
	}
	if value := requestFormValue(c, "ELECTRUM_SERVER_URL"); value != "" {
		formValues.ElectrumServerURL = value
	}
	if value := requestFormValue(c, "ESPLORA_SERVER_URL"); value != "" {
		formValues.EsploraServerURL = value
	}

	return formValues
}

func requestFormValue(c *gin.Context, key string) string {
	return strings.TrimSpace(c.Request.FormValue(key))
}

func normalizeLDKChainSourceType(chainSourceType string) string {
	if strings.EqualFold(strings.TrimSpace(chainSourceType), string(ldk.ChainSourceEsplora)) {
		return string(ldk.ChainSourceEsplora)
	}
	if strings.EqualFold(strings.TrimSpace(chainSourceType), string(ldk.ChainSourceElectrum)) {
		return string(ldk.ChainSourceElectrum)
	}

	return string(ldk.ChainSourceBitcoind)
}
