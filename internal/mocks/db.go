package mocks

import (
	"github.com/dgrslabs/bucket/pkg/logger"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockDB struct {
	mock.Mock
	*gorm.DB
	*logger.BucketLogger
}

func (m *MockDB) Close() {
	m.Called()
}

func (m *MockDB) Migrate() error {
	args := m.Called()
	return args.Error(0)
}
