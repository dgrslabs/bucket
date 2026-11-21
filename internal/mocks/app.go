package mocks

import (
	"net/http"

	"github.com/dgrslabs/bucket/internal/model"
)

type MockBucket struct {
	fileService   *FileService
	folderService *FolderService
}

func NewMockBucket() *MockBucket {
	return &MockBucket{
		fileService:   new(FileService),
		folderService: new(FolderService),
	}
}

func (m *MockBucket) UploadFile(user_id, parent_id, file_name, content_type string, file_data []byte) (string, error) {
	args := m.fileService.Called(user_id, parent_id, file_name, content_type, file_data)

	return args.Get(0).(string), args.Error(1)
}

func (m *MockBucket) GetFile(fileID string) (*model.FileModel, error) {
	args := m.fileService.Called(fileID)
	return args.Get(0).(*model.FileModel), args.Error(1)
}

func (m *MockBucket) ListFiles(parentID string) ([]model.FileModel, error) {
	args := m.fileService.Called(parentID)
	return args.Get(0).([]model.FileModel), args.Error(1)
}

func (m *MockBucket) MoveFile(fileID, newParentID string) error {
	args := m.fileService.Called(fileID, newParentID)
	return args.Error(0)
}

func (m *MockBucket) RenameFile(fileID, newName string) error {
	args := m.fileService.Called(fileID, newName)
	return args.Error(0)
}

func (m *MockBucket) DeleteFile(fileID string) (string, error) {
	args := m.fileService.Called(fileID)
	return args.String(0), args.Error(1)
}

func (m *MockBucket) ScrubFile(fileID string) (string, error) {
	args := m.fileService.Called(fileID)
	return args.String(0), args.Error(1)
}

func (m *MockBucket) NewFolder(userID, parentID, folderName, description string) (string, error) {
	args := m.folderService.Called(userID, parentID, folderName, description)
	return args.String(0), args.Error(1)
}

func (m *MockBucket) ListFolders(parentID string) ([]model.FolderModel, error) {
	args := m.folderService.Called(parentID)
	return args.Get(0).([]model.FolderModel), args.Error(1)
}

func (m *MockBucket) GetFolderWithContent(userID, folderID string) (*model.FolderModel, error) {
	args := m.folderService.Called(userID, folderID)
	return args.Get(0).(*model.FolderModel), args.Error(1)
}

func (m *MockBucket) MoveFolder(folderID, newParentID string) error {
	args := m.folderService.Called(folderID, newParentID)
	return args.Error(0)
}

func (m *MockBucket) RenameFolder(userID, folderID, newName string) error {
	args := m.folderService.Called(userID, folderID, newName)
	return args.Error(0)
}

func (m *MockBucket) DeleteFolder(folderID string) (string, error) {
	args := m.folderService.Called(folderID)
	return args.String(0), args.Error(1)
}

func (m *MockBucket) DeleteFolderPermanently(userID, folderID string) (string, error) {
	args := m.folderService.Called(userID, folderID)
	return args.String(0), args.Error(1)
}

func (m *MockBucket) GetHandler() http.Handler {
	args := m.folderService.Called()
	return args.Get(0).(http.Handler)
}

func (m *MockBucket) StartServer(port string) error {
	args := m.folderService.Called(port)
	return args.Error(0)
}

func (m *MockBucket) Close() {
	m.folderService.Called()
}
