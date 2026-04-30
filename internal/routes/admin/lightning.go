package admin

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/internal/lightning/ldk"
	m "github.com/lescuer97/nutmix/internal/mint"
	"github.com/lescuer97/nutmix/internal/routes/admin/templates"
)

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
