package mockdb

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (pql *MockDB) GetConfig(tx pgx.Tx) (utils.Config, error) {
	_ = tx
	if pql.GetConfigErr != nil {
		return utils.Config{}, databaseError(fmt.Errorf("getting config: %w", pql.GetConfigErr))
	}

	return pql.Config, nil
}

func (pql *MockDB) SetConfig(tx pgx.Tx, config utils.Config) error {
	_ = tx
	pql.Config = config
	return nil
}

func (pql *MockDB) UpdateConfig(tx pgx.Tx, config utils.Config) error {
	_ = tx
	pql.Config = config
	return nil
}

func (pql *MockDB) GetNostrNotificationConfig(tx pgx.Tx) (*utils.NostrNotificationConfig, error) {
	_ = tx
	return pql.NostrNotificationConfig, nil
}

func (pql *MockDB) UpdateNostrNotificationConfig(tx pgx.Tx, config utils.NostrNotificationConfig) error {
	_ = tx
	if pql.UpdateNostrNotificationConfigErr != nil {
		return pql.UpdateNostrNotificationConfigErr
	}

	updatedConfig := config
	pql.NostrNotificationConfig = &updatedConfig
	return nil
}
