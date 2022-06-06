package persistence

import (
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3" // Blank import to register the sqlite3 driver
	"github.com/status-im/go-waku/tests"
	"github.com/status-im/go-waku/waku/v2/protocol"
	"github.com/status-im/go-waku/waku/v2/utils"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func NewMock() *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		utils.Logger().Fatal("opening a stub database connection", zap.Error(err))
	}

	return db
}

func TestDbStore(t *testing.T) {
	db := NewMock()
	option := WithDB(db)
	store, err := NewDBStore(utils.Logger(), option)
	require.NoError(t, err)

	res, err := store.GetAll()
	require.NoError(t, err)
	require.Empty(t, res)

	err = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test", 1), utils.GetUnixEpoch(), "test"))
	require.NoError(t, err)

	res, err = store.GetAll()
	require.NoError(t, err)
	require.NotEmpty(t, res)
}

func TestStoreRetention(t *testing.T) {
	db := NewMock()
	store, err := NewDBStore(utils.Logger(), WithDB(db), WithRetentionPolicy(5, 20*time.Second))
	require.NoError(t, err)

	insertTime := time.Now()

	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test1", insertTime.Add(-70*time.Second).UnixNano()), insertTime.Add(-70*time.Second).UnixNano(), "test"))
	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test2", insertTime.Add(-60*time.Second).UnixNano()), insertTime.Add(-60*time.Second).UnixNano(), "test"))
	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test3", insertTime.Add(-50*time.Second).UnixNano()), insertTime.Add(-50*time.Second).UnixNano(), "test"))
	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test4", insertTime.Add(-40*time.Second).UnixNano()), insertTime.Add(-40*time.Second).UnixNano(), "test"))
	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test5", insertTime.Add(-30*time.Second).UnixNano()), insertTime.Add(-30*time.Second).UnixNano(), "test"))

	dbResults, err := store.GetAll()
	require.NoError(t, err)
	require.Len(t, dbResults, 5)

	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test6", insertTime.Add(-20*time.Second).UnixNano()), insertTime.Add(-20*time.Second).UnixNano(), "test"))
	_ = store.Put(protocol.NewEnvelope(tests.CreateWakuMessage("test7", insertTime.Add(-10*time.Second).UnixNano()), insertTime.Add(-10*time.Second).UnixNano(), "test"))

	// This step simulates starting go-waku again from scratch

	store, err = NewDBStore(utils.Logger(), WithDB(db), WithRetentionPolicy(5, 40*time.Second))
	require.NoError(t, err)

	dbResults, err = store.GetAll()
	require.NoError(t, err)
	require.Len(t, dbResults, 3)
	require.Equal(t, "test5", dbResults[0].Message.ContentTopic)
	require.Equal(t, "test6", dbResults[1].Message.ContentTopic)
	require.Equal(t, "test7", dbResults[2].Message.ContentTopic)
}
