package ldk

import (
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	ldk_node "github.com/lescuer97/ldkgo/bindings/ldk_node_ffi"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/utils"
)

var errOnchainSendValidation = errors.New("on-chain send validation failed")

func IsOnchainSendValidationError(err error) bool {
	return errors.Is(err, errOnchainSendValidation)
}

type LDKBalances struct {
	TotalOnchainSats     uint64
	AvailableOnchainSats uint64
	LightningSats        uint64
}

func (l *LDK) WalletBalance() (cashu.Amount, error) {
	balances, err := l.Balances()
	if err != nil {
		return cashu.Amount{}, err
	}

	return cashu.Amount{
		Unit:   cashu.Sat,
		Amount: balances.LightningSats,
	}, nil
}

func (l *LDK) Balances() (LDKBalances, error) {
	node, err := l.getNode()
	if err != nil {
		return LDKBalances{}, err
	}

	return mapLDKBalances(node.ListBalances()), nil
}

func (l *LDK) SyncWallets() error {
	node, err := l.getNode()
	if err != nil {
		return err
	}

	if err := node.SyncWallets(); err != nil {
		return fmt.Errorf("node.SyncWallets(): %w", err)
	}

	return nil
}

func mapLDKBalances(balance ldk_node.BalanceDetails) LDKBalances {
	return LDKBalances{
		TotalOnchainSats:     balance.TotalOnchainBalanceSats,
		AvailableOnchainSats: balance.SpendableOnchainBalanceSats,
		LightningSats:        balance.TotalLightningBalanceSats,
	}
}

func (l *LDK) LightningType() Backend {
	return LDKNODE
}

func (l *LDK) GetNetwork() *chaincfg.Params {
	if l != nil && l.network != "" {
		if chainParams, err := utils.CheckChainParams(l.network); err == nil {
			return &chainParams
		}
	}

	node, err := l.getNode()
	if err != nil {
		return &chaincfg.MainNetParams
	}
	return mapLDKNetwork(node.Config().Network)
}

func mapLDKNetwork(network ldk_node.Network) *chaincfg.Params {
	switch network {
	case ldk_node.NetworkBitcoin:
		return &chaincfg.MainNetParams
	case ldk_node.NetworkTestnet:
		return &chaincfg.TestNet3Params
	case ldk_node.NetworkSignet:
		return &chaincfg.SigNetParams
	case ldk_node.NetworkRegtest:
		return &chaincfg.RegressionNetParams
	default:
		return &chaincfg.MainNetParams
	}
}

func (l *LDK) ActiveMPP() bool {
	return true
}

func (l *LDK) VerifyUnitSupport(unit cashu.Unit) bool {
	return unit == cashu.Sat || unit == cashu.Msat
}

func (l *LDK) DescriptionSupport() bool {
	return true
}

func (l *LDK) NewOnchainAddress() (string, error) {
	node, err := l.getNode()
	if err != nil {
		return "", err
	}

	address, err := node.OnchainPayment().NewAddress()
	if err != nil {
		return "", fmt.Errorf("node.OnchainPayment().NewAddress(): %w", err)
	}

	return address, nil
}

func (l *LDK) SendOnchain(address string, sats uint64) error {
	node, err := l.getNode()
	if err != nil {
		return err
	}

	balances := mapLDKBalances(node.ListBalances())
	if err := validateOnchainSendAddress(address, l.GetNetwork()); err != nil {
		return err
	}
	if err := validateOnchainSendAmount(sats, balances.AvailableOnchainSats); err != nil {
		return err
	}

	_, err = node.OnchainPayment().SendToAddress(address, sats, nil)
	if err != nil {
		return fmt.Errorf("node.OnchainPayment().SendToAddress(): %w", err)
	}

	return nil
}

func validateOnchainSendAmount(amount uint64, available uint64) error {
	if available == 0 {
		return fmt.Errorf("%w: available on-chain balance is too low to send funds", errOnchainSendValidation)
	}
	if amount == 0 {
		return fmt.Errorf("%w: sats amount must be greater than 0", errOnchainSendValidation)
	}
	if amount > available {
		return fmt.Errorf("%w: sats amount exceeds available on-chain balance (%d sats)", errOnchainSendValidation, available)
	}
	return nil
}

func validateOnchainSendAddress(raw string, network *chaincfg.Params) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fmt.Errorf("%w: bitcoin address is required", errOnchainSendValidation)
	}

	if network == nil {
		network = &chaincfg.MainNetParams
	}

	address, err := btcutil.DecodeAddress(value, network)
	if err == nil {
		if !address.IsForNet(network) {
			return fmt.Errorf("%w: bitcoin address does not match the active %s network", errOnchainSendValidation, network.Name)
		}
		return nil
	}

	for _, candidate := range []*chaincfg.Params{
		&chaincfg.MainNetParams,
		&chaincfg.TestNet3Params,
		&chaincfg.SigNetParams,
		&chaincfg.RegressionNetParams,
	} {
		decoded, decodeErr := btcutil.DecodeAddress(value, candidate)
		if decodeErr != nil {
			continue
		}
		if !decoded.IsForNet(network) {
			return fmt.Errorf("%w: bitcoin address does not match the active %s network", errOnchainSendValidation, network.Name)
		}
		return nil
	}

	return fmt.Errorf("%w: bitcoin address is invalid", errOnchainSendValidation)
}
