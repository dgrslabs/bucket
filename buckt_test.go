package bucket

import (
	"bytes"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/dgrslabs/bucket/internal/backend"
	"github.com/dgrslabs/bucket/internal/database"
	"github.com/dgrslabs/bucket/internal/domain"
	"github.com/dgrslabs/bucket/internal/mocks"
	"github.com/dgrslabs/bucket/internal/model"
	"github.com/dgrslabs/bucket/pkg/events"
	"github.com/dgrslabs/bucket/pkg/imageproc"
	"github.com/dgrslabs/bucket/pkg/metrics"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/* ------------------------------------------------------------------ */
/* Shared real-DB test harness                                        */
/*                                                                    */
/* newTestClient builds a real Client (real services, not mocks) for  */
/* the integration-style suites in the bucket_*_test.go files. It      */
/* replaces the five near-identical per-file constructors these tests */
/* used to carry. Defaults: SQLite (temp file), temp media dir,       */
/* silent logs, flat namespaces, no size limit. Override via options. */
/* ------------------------------------------------------------------ */

type testClientConfig struct {
	driver         DBDrivers
	flatNameSpaces bool
	maxFileSize    int64
	metrics        metrics.Recorder
	eventHandlers  []events.Handler
	dedup          bool
	derivatives    []DerivativeSpec
	imageProcessor imageproc.Processor
}

type testClientOption func(*testClientConfig)

// withPostgres runs the Client against the Postgres in BUCKT_PG_DSN (the test
// skips when it is unset). Nested + Postgres is the production configuration.
func withPostgres() testClientOption { return func(c *testClientConfig) { c.driver = Postgres } }

// withNested uses nested-namespace mode so trash/move/restore exercise physical
// backend blob moves, not just DB path rewrites.
func withNested() testClientOption { return func(c *testClientConfig) { c.flatNameSpaces = false } }

// withMaxFileSize caps the upload size (used to trigger ErrFileTooLarge).
func withMaxFileSize(n int64) testClientOption {
	return func(c *testClientConfig) { c.maxFileSize = n }
}

// withMetrics attaches a metrics recorder so backend ops are collected.
func withMetrics(m metrics.Recorder) testClientOption {
	return func(c *testClientConfig) { c.metrics = m }
}

// withEventHandler registers a lifecycle event handler on the test client.
func withEventHandler(h events.Handler) testClientOption {
	return func(c *testClientConfig) { c.eventHandlers = append(c.eventHandlers, h) }
}

// withDedup enables content deduplication on the test client.
func withDedup() testClientOption {
	return func(c *testClientConfig) { c.dedup = true }
}

// withImageDerivatives configures resized-image variants on the test client.
func withImageDerivatives(specs ...DerivativeSpec) testClientOption {
	return func(c *testClientConfig) { c.derivatives = append(c.derivatives, specs...) }
}

// withImageProcessor sets a custom image processor on the test client.
func withImageProcessor(p imageproc.Processor) testClientOption {
	return func(c *testClientConfig) { c.imageProcessor = p }
}

// pgDSN returns the Postgres DSN from BUCKT_PG_DSN or skips the test.
func pgDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("BUCKT_PG_DSN")
	if dsn == "" {
		t.Skip("BUCKT_PG_DSN not set; skipping Postgres integration test")
	}
	return dsn
}

// dropBucketTables clears any existing bucket schema so a Postgres test starts
// from a fresh slate (and re-runs the migrations cleanly).
func dropBucketTables(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	for _, tbl := range []string{"file_models", "folder_models", "bucket_schema_migrations", "bucket_migration_models"} {
		_, err := sqlDB.Exec("DROP TABLE IF EXISTS " + tbl + " CASCADE")
		require.NoError(t, err)
	}
}

func newTestClient(t *testing.T, opts ...testClientOption) *Client {
	t.Helper()

	cfg := testClientConfig{driver: SQLite, flatNameSpaces: true}
	for _, o := range opts {
		o(&cfg)
	}

	dir := t.TempDir()

	var sqlDB *sql.DB
	var err error
	switch cfg.driver {
	case Postgres:
		sqlDB, err = sql.Open("postgres", pgDSN(t))
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
		dropBucketTables(t, sqlDB)
	default:
		sqlDB, err = sql.Open("sqlite3", filepath.Join(dir, "bucket.db"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = sqlDB.Close() })
	}

	client, err := New(Config{
		DB:             DBConfig{Driver: cfg.driver, Database: sqlDB},
		MediaDir:       filepath.Join(dir, "media"),
		Log:            LogConfig{Silence: true},
		FlatNameSpaces: cfg.flatNameSpaces,
		MaxFileSize:    cfg.maxFileSize,
		Metrics:        cfg.metrics,
		EventHandlers:  cfg.eventHandlers,
		Dedup:          cfg.dedup,
		Derivatives:    cfg.derivatives,
		ImageProcessor: cfg.imageProcessor,
	})
	require.NoError(t, err) // also proves the migrations run cleanly on this driver
	t.Cleanup(func() { _ = client.Close() })
	return client
}

type MockBucket struct {
	*Client
	MockFileService   *mocks.FileService
	MockFolderService *mocks.FolderService
}

// memSQLite opens a throwaway in-memory SQLite DB and registers its cleanup.
func memSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// setup builds a Client from the given config and swaps in mock file/folder
// services. Client cleanup is registered so callers don't repeat it.
func setup(t *testing.T, bucketOpts Config) MockBucket {
	bucket, err := New(bucketOpts)
	require.NoError(t, err)
	require.NotNil(t, bucket)
	t.Cleanup(func() { _ = bucket.Close() })

	mockFileService := new(mocks.FileService)
	mockFolderService := new(mocks.FolderService)

	bucket.fileService = mockFileService
	bucket.folderService = mockFolderService

	return MockBucket{
		Client:            bucket,
		MockFileService:   mockFileService,
		MockFolderService: mockFolderService,
	}
}

// setupBucketTest returns a mock-backed Client (in-memory SQLite). Cleanup for
// both the DB and the Client is registered via t.Cleanup.
func setupBucketTest(t *testing.T) MockBucket {
	return setup(t, Config{
		DB:             DBConfig{Driver: SQLite, Database: memSQLite(t)},
		Log:            LogConfig{LogTerminal: false, Silence: false},
		MediaDir:       "media",
		FlatNameSpaces: false,
	})
}

func TestNew(t *testing.T) {
	t.Run("SQLite with provided instance", func(t *testing.T) {
		mockBucket := setupBucketTest(t)

		assert.NotNil(t, mockBucket)
	})

	t.Run("SQLite without provided instance", func(t *testing.T) {
		sqlDB := memSQLite(t)

		bucketOpts := Config{
			DB:             DBConfig{Driver: SQLite, Database: sqlDB},
			Log:            LogConfig{LogTerminal: false, Silence: false},
			MediaDir:       "media",
			FlatNameSpaces: false,
		}

		bucket, err := New(bucketOpts)
		assert.NoError(t, err)
		assert.NotNil(t, bucket)
	})

	// t.Run("Postgres with provided instance", func(t *testing.T) {
	// 	sqlDB, err := sql.Open("postgres", "user=postgres password=postgres dbname=postgres sslmode=disable")
	// 	assert.NoError(t, err)
	// 	defer func() { _ = sqlDB.Close() }()

	// 	bucketOpts := Config{
	// 		DB:             DBConfig{Driver: Postgres, Database: sqlDB},
	// 		Log:            LogConfig{LogTerminal: false, Silence: false},
	// 		MediaDir:       "media",
	//
	// 		FlatNameSpaces: false,
	// 	}

	// 	bucket, err := New(bucketOpts)
	// 	// assert.NoError(t, err)
	// 	// assert.NotNil(t, bucket)
	// 	assert.Error(t, err) // 🚨 Error: pq: database "postgres" does not exist
	// 	assert.Nil(t, bucket)
	// })

	t.Run("Postgres without provided instance", func(t *testing.T) {
		bucketOpts := Config{
			DB:       DBConfig{Driver: Postgres, Database: nil},
			Log:      LogConfig{LogTerminal: false, Silence: false},
			MediaDir: "media",

			FlatNameSpaces: false,
		}

		bucket, err := New(bucketOpts)
		assert.Error(t, err)
		assert.Nil(t, bucket)
	})

}

// requireDefault builds a Client via Default with the given options, asserts it
// succeeded, and registers its cleanup.
func requireDefault(t *testing.T, opts ...ConfigFunc) {
	t.Helper()
	bucket, err := Default(opts...)
	require.NoError(t, err)
	require.NotNil(t, bucket)
	t.Cleanup(func() { _ = bucket.Close() })
}

// noopCacheConfig is the standard test cache (no-op manager + a valid ristretto
// file-cache config).
func noopCacheConfig() CacheConfig {
	return CacheConfig{
		Manager: mocks.NewNoopCache(),
		FileCacheConfig: FileCacheConfig{
			NumCounters: 1e7,     // 10M
			MaxCost:     1 << 30, // 1GB
			BufferItems: 64,
		},
	}
}

func TestDefault(t *testing.T) {
	t.Run("With Default", func(t *testing.T) { requireDefault(t) })
	t.Run("With FlatNameSpaces", func(t *testing.T) { requireDefault(t, FlatNameSpaces(true)) })
	t.Run("With MediaDir", func(t *testing.T) { requireDefault(t, MediaDir("media")) })
	t.Run("With Cache", func(t *testing.T) { requireDefault(t, WithCache(noopCacheConfig())) })
	t.Run("With Log", func(t *testing.T) {
		requireDefault(t, WithLog(LogConfig{LogTerminal: false, Silence: false}))
	})
	t.Run("With DB", func(t *testing.T) { requireDefault(t, WithDB(SQLite, memSQLite(t))) })
	t.Run("With all options", func(t *testing.T) {
		requireDefault(t,
			FlatNameSpaces(true),
			MediaDir("media"),
			WithCache(noopCacheConfig()),
			WithLog(LogConfig{LogTerminal: false, Silence: false}),
			WithDB(SQLite, memSQLite(t)),
		)
	})
}

func TestClose(t *testing.T) {
	bucket := setupBucketTest(t)
	assert.NotNil(t, bucket)

	assert.NoError(t, bucket.Close())
}

func TestNewFolder(t *testing.T) {
	bucket := setupBucketTest(t)

	// Expected folder ID
	expectedFolderID := "550e8400-e29b-41d4-a716-446655440000"

	// Mocking the CreateFolder method
	bucket.MockFolderService.On("CreateFolder", "user1", "550e8400-e29b-41d4-a716-446655440001", "folder1", "description1").
		Return(expectedFolderID, nil)

	// Call the method
	actualFolderID, err := bucket.NewFolder("user1", "550e8400-e29b-41d4-a716-446655440001", "folder1", "description1")

	// Assertions
	assert.NoError(t, err)
	assert.Equal(t, expectedFolderID, actualFolderID) // Compare the expected and actual values

	// Verify mock expectations
	bucket.MockFolderService.AssertExpectations(t)
}

func TestListFolders(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	expectedFolders := []model.FolderModel{
		{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), Name: "folder1", Description: "description1"},
		{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"), Name: "folder2", Description: "description2"},
	}

	bucket.MockFolderService.On("GetFolders", "550e8400-e29b-41d4-a716-446655440002").
		Return(expectedFolders, nil)

	// Call the method
	folders, err := bucket.ListFolders("550e8400-e29b-41d4-a716-446655440002")
	assert.NoError(t, err)
	assert.Equal(t, expectedFolders, folders)

	// Verify expectations
	bucket.MockFolderService.AssertExpectations(t)
}

func TestGetFolderWithContent(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	expectedFolder := model.FolderModel{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Name:        "folder1",
		Description: "description1",
		Files: []model.FileModel{
			{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"), Name: "file1", ContentType: "text/plain", Data: []byte("file content")},
			{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440002"), Name: "file2", ContentType: "text/plain", Data: []byte("file content")},
		},
		Folders: []model.FolderModel{
			{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440003"), Name: "folder2", Description: "description2"},
			{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440004"), Name: "folder3", Description: "description3"},
		},
	}

	bucket.MockFolderService.On("GetFolder", "user1", "550e8400-e29b-41d4-a716-446655440000").
		Return(&expectedFolder, nil)

	// Call the method
	folder, err := bucket.GetFolderWithContent("user1", "550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)
	assert.NotNil(t, folder)

	// Verify expectations
	bucket.MockFolderService.AssertExpectations(t)
}

func TestMoveFolder(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFolderService.On("MoveFolder", "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001").
		Return(nil)

	// Call the method
	err := bucket.MoveFolder("user1", "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001")
	assert.NoError(t, err)

	// Verify expectations
	bucket.MockFolderService.AssertExpectations(t)

}

func TestDeleteFolder(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFolderService.On("DeleteFolder", "550e8400-e29b-41d4-a716-446655440000").
		Return("550e8400-e29b-41d4-a716-446655440001", nil)

	// Call the method
	_, err := bucket.DeleteFolder("550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)

	// Verify expectations
	bucket.MockFolderService.AssertExpectations(t)
}

func TestDeleteFolderPermanently(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFolderService.On("ScrubFolder", "user1", "550e8400-e29b-41d4-a716-446655440000").
		Return("parent1", nil)

	// Call the method
	_, err := bucket.DeleteFolderPermanently("user1", "550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)

	// Verify expectations
	bucket.MockFolderService.AssertExpectations(t)
}

func TestUploadFile(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFileService.On("CreateFile", "user1", "folder1", "file1", "text/plain", []byte("file content")).
		Return("550e8400-e29b-41d4-a716-446655440000", nil)

	// Call the method
	fileID, err := bucket.UploadFile("user1", "folder1", "file1", "text/plain", []byte("file content"))
	assert.NoError(t, err)
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", fileID)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestGetFile(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	expectedFile := model.FileModel{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Name:        "file1",
		ContentType: "text/plain",
		Data:        []byte("file content"),
	}
	bucket.MockFileService.On("GetFile", "550e8400-e29b-41d4-a716-446655440000").
		Return(&expectedFile, nil)

	// Call the method
	file, err := bucket.GetFile("550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)
	assert.NotNil(t, file)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestGetFileStream(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	expectedFile := model.FileModel{
		ID:          uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"),
		Name:        "file1",
		ContentType: "text/plain",
		Data:        []byte("file content"),
	}
	expectedStream := io.NopCloser(bytes.NewReader([]byte("file content")))

	bucket.MockFileService.On("GetFileStream", "550e8400-e29b-41d4-a716-446655440000").
		Return(&expectedFile, expectedStream, nil)

	// Call the method
	file, stream, err := bucket.GetFileStream("550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)
	assert.NotNil(t, file)
	assert.NotNil(t, stream)
	defer func() { _ = stream.Close() }()

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestListFilesMetadata(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	expectedFiles := []model.FileModel{
		{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), Name: "file1", ContentType: "text/plain", Data: []byte("file content")},
		{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"), Name: "file2", ContentType: "text/plain", Data: []byte("file content")},
	}

	bucket.MockFileService.On("GetFilesMetadata", "550e8400-e29b-41d4-a716-446655440002").
		Return(expectedFiles, nil)

	// Call the method
	files, err := bucket.ListFilesMetadata("550e8400-e29b-41d4-a716-446655440002")
	assert.NoError(t, err)
	assert.Equal(t, expectedFiles, files)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestListFiles(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	expectedFiles := []model.FileModel{
		{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440000"), Name: "file1", ContentType: "text/plain", Data: []byte("file content")},
		{ID: uuid.MustParse("550e8400-e29b-41d4-a716-446655440001"), Name: "file2", ContentType: "text/plain", Data: []byte("file content")},
	}

	bucket.MockFileService.On("GetFiles", "550e8400-e29b-41d4-a716-446655440002").
		Return(expectedFiles, nil)

	// Call the method
	files, err := bucket.ListFiles("550e8400-e29b-41d4-a716-446655440002")
	assert.NoError(t, err)
	assert.Equal(t, expectedFiles, files)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestMoveFile(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFileService.On("MoveFile", "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001").
		Return(nil)

	// Call the method
	err := bucket.MoveFile("550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001")
	assert.NoError(t, err)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestDeleteFile(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFileService.On("DeleteFile", "550e8400-e29b-41d4-a716-446655440000").
		Return("parent1", nil)

	// Call the method
	_, err := bucket.DeleteFile("550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestDeleteFilePermanently(t *testing.T) {
	bucket := setupBucketTest(t)

	// Mock the expected behavior
	bucket.MockFileService.On("ScrubFile", "550e8400-e29b-41d4-a716-446655440000").
		Return("parent1", nil)

	// Call the method
	_, err := bucket.DeleteFilePermanently("550e8400-e29b-41d4-a716-446655440000")
	assert.NoError(t, err)

	// Verify expectations
	bucket.MockFileService.AssertExpectations(t)
}

func TestInitializeCache(t *testing.T) {
	// Mock logger
	mockLogger := &mocks.NoopLogger{}

	t.Run("returns provided cache manager and valid lruCache", func(t *testing.T) {
		fileCacheConfig := FileCacheConfig{
			NumCounters: 1e3,
			MaxCost:     1 << 20,
			BufferItems: 8,
		}
		providedCache := mocks.NewNoopCache()
		conf := CacheConfig{
			Manager:         providedCache,
			FileCacheConfig: fileCacheConfig,
		}

		cm, lru := initializeCache(conf, mockLogger)
		assert.Equal(t, providedCache, cm)
		assert.NotNil(t, lru)
	})

	t.Run("returns NoOpCache if Manager is nil", func(t *testing.T) {
		fileCacheConfig := FileCacheConfig{
			NumCounters: 1e3,
			MaxCost:     1 << 20,
			BufferItems: 8,
		}
		conf := CacheConfig{
			Manager:         nil,
			FileCacheConfig: fileCacheConfig,
		}

		cm, lru := initializeCache(conf, mockLogger)
		assert.NotNil(t, cm)
		assert.NotNil(t, lru)
	})

	t.Run("fallbacks to NoOpFileCache on error", func(t *testing.T) {
		// Use invalid config to force error
		fileCacheConfig := FileCacheConfig{
			NumCounters: 0,
			MaxCost:     0,
			BufferItems: 0,
		}
		conf := CacheConfig{
			Manager:         nil,
			FileCacheConfig: fileCacheConfig,
		}

		cm, lru := initializeCache(conf, mockLogger)
		assert.NotNil(t, cm)
		assert.NotNil(t, lru)
	})
}

func TestResolveBackend(t *testing.T) {
	mockLogger := &mocks.NoopLogger{}
	mockLRU := &mocks.NoopLRUCache{}
	mediaDir := "media"

	t.Run("MigrationEnabled with Source and Target", func(t *testing.T) {
		source := &mocks.Backend{NameVal: "local"}
		target := &mocks.Backend{NameVal: "mock"}
		bc := BackendConfig{
			MigrationEnabled: true,
			Source:           source,
			Target:           target,
		}
		result := resolveBackend(mediaDir, bc, mockLogger, mockLRU, nil, nil)
		_, ok := result.(*backend.MigrationBackendService)
		assert.True(t, ok)
	})

	t.Run("Source only - placeholder", func(t *testing.T) {
		source := &domain.PlaceholderBackend{Title: "local"}
		bc := BackendConfig{
			Source: source,
		}
		result := resolveBackend(mediaDir, bc, mockLogger, mockLRU, nil, nil)
		// Placeholder should be replaced with real local backend
		_, ok := result.(*backend.LocalFileSystemService)
		assert.True(t, ok)
	})

	t.Run("Source only - real backend passes through", func(t *testing.T) {
		source := &mocks.Backend{NameVal: "s3"}
		bc := BackendConfig{
			Source: source,
		}
		result := resolveBackend(mediaDir, bc, mockLogger, mockLRU, nil, nil)
		assert.Equal(t, source, result)
	})

	t.Run("Target only - placeholder", func(t *testing.T) {
		target := &domain.PlaceholderBackend{Title: "local"}
		bc := BackendConfig{
			Target: target,
		}
		result := resolveBackend(mediaDir, bc, mockLogger, mockLRU, nil, nil)
		_, ok := result.(*backend.LocalFileSystemService)
		assert.True(t, ok)
	})

	t.Run("No Source or Target", func(t *testing.T) {
		bc := BackendConfig{}
		result := resolveBackend(mediaDir, bc, mockLogger, mockLRU, nil, nil)
		_, ok := result.(*backend.LocalFileSystemService)
		assert.True(t, ok)
	})
}

func TestResolveIfPlaceholder(t *testing.T) {
	mockLogger := &mocks.NoopLogger{}
	mockLRU := &mocks.NoopLRUCache{}
	mediaDir := "media"

	t.Run("Returns LocalFileSystemService if backend is a PlaceholderBackend", func(t *testing.T) {
		b := &domain.PlaceholderBackend{Title: "local"}
		result := resolveIfPlaceholder(b, mediaDir, mockLogger, mockLRU)
		_, ok := result.(*backend.LocalFileSystemService)
		assert.True(t, ok)
	})

	t.Run("Returns backend as is if it is not a PlaceholderBackend", func(t *testing.T) {
		b := &mocks.Backend{NameVal: "mock"}
		result := resolveIfPlaceholder(b, mediaDir, mockLogger, mockLRU)
		assert.Equal(t, b, result)
	})
}

func TestNewAppServices(t *testing.T) {
	mockLogger := &mocks.NoopLogger{}
	mockCacheManager := &mocks.NoopCache{}
	mockBackend := &mocks.Backend{}
	// Use an in-memory SQLite DB for testing
	sqlDB := memSQLite(t)

	dbConf := DBConfig{Driver: SQLite, Database: sqlDB}
	db, err := database.NewDB(dbConf.Database, dbConf.Driver, mockLogger, false, "")
	assert.NoError(t, err)
	defer func() { _ = db.Close() }()

	t.Run("returns valid FolderService and FileService", func(t *testing.T) {
		folderService, fileService := newAppServices(
			true,
			DefaultMaxFileSize,
			DefaultMaxTrashBatchSize,
			DefaultBackendOpTimeout,
			db,
			mockLogger,
			mockCacheManager,
			mockBackend,
			nil,
			false,
			nil,
		)
		assert.NotNil(t, folderService)
		assert.NotNil(t, fileService)
	})

	t.Run("returns different instances for FolderService and FileService", func(t *testing.T) {
		folderService, fileService := newAppServices(
			false,
			DefaultMaxFileSize,
			DefaultMaxTrashBatchSize,
			DefaultBackendOpTimeout,
			db,
			mockLogger,
			mockCacheManager,
			mockBackend,
			nil,
			false,
			nil,
		)
		assert.NotNil(t, folderService)
		assert.NotNil(t, fileService)
		assert.NotEqual(t, folderService, fileService)
	})
}
