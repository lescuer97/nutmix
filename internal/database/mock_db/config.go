package mockdb

import (
	"github.com/lescuer97/nutmix/internal/utils"
)

func (pql *MockDB) GetConfig() (utils.Config, error) {
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
