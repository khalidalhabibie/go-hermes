package testkit

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type PostgresTestDB struct {
	DB     *gorm.DB
	Schema string
}

func NewPostgresTestDB(t *testing.T) *PostgresTestDB {
	t.Helper()

	baseDSN := os.Getenv("TEST_DATABASE_DSN")
	if baseDSN == "" {
		t.Skip("TEST_DATABASE_DSN is not set")
	}

	bootstrapDB := openPostgresDB(t, baseDSN)
	schema := "test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	require.NoError(t, bootstrapDB.Exec("CREATE SCHEMA "+schema).Error)

	testDB := openPostgresDB(t, appendSearchPath(baseDSN, schema))
	applySQLMigrations(t, testDB)

	t.Cleanup(func() {
		sqlDB, err := testDB.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())

		require.NoError(t, bootstrapDB.Exec("DROP SCHEMA IF EXISTS "+schema+" CASCADE").Error)

		bootstrapSQLDB, err := bootstrapDB.DB()
		require.NoError(t, err)
		require.NoError(t, bootstrapSQLDB.Close())
	})

	return &PostgresTestDB{
		DB:     testDB,
		Schema: schema,
	}
}

func openPostgresDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	return db
}

func applySQLMigrations(t *testing.T, db *gorm.DB) {
	t.Helper()

	migrationFiles, err := filepath.Glob(filepath.Join(repoRoot(t), "migrations", "*.up.sql"))
	require.NoError(t, err)
	sort.Strings(migrationFiles)

	for _, migrationFile := range migrationFiles {
		content, err := os.ReadFile(migrationFile)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(content)).Error, migrationFile)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
}

func appendSearchPath(dsn, schema string) string {
	if strings.Contains(dsn, "://") {
		if strings.Contains(dsn, "?") {
			return dsn + "&search_path=" + schema
		}
		return dsn + "?search_path=" + schema
	}
	return dsn + " search_path=" + schema
}
