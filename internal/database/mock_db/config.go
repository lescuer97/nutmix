package mockdb

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/lescuer97/nutmix/internal/utils"
)

func (pql *MockDB) GetConfig() (utils.Config, error) {
	if pql.GetConfigErr != nil {
		return utils.Config{}, databaseError(fmt.Errorf("getting config: %w", pql.GetConfigErr))
	}

	return pql.Config, nil
}

func (pql *MockDB) SetConfig(config utils.Config) error {
	pql.Config = config
	return nil
}

func (pql *MockDB) UpdateConfig(config utils.Config) error {
	pql.Config = config
	return nil
}

func (pql *MockDB) UpdateNostrNotificationConfig(config utils.Config) error {
	if pql.UpdateNostrNotificationConfigErr != nil {
		return pql.UpdateNostrNotificationConfigErr
	}

	pql.Config.NOSTR_NOTIFICATION_NPUBS = config.NOSTR_NOTIFICATION_NPUBS
	pql.Config.NOSTR_NOTIFICATIONS = config.NOSTR_NOTIFICATIONS
	pql.Config.NOSTR_NOTIFICATION_NIP04_DM = config.NOSTR_NOTIFICATION_NIP04_DM
	return nil
}

func (pql *MockDB) UpdateNostrNotificationConfigTx(tx pgx.Tx, config utils.Config) error {
	_ = tx
	return pql.UpdateNostrNotificationConfig(config)
}
