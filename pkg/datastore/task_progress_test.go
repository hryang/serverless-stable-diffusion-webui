package datastore

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTaskProgressDatastore(t *testing.T) {
	t.Run("Test NewTaskProgressDatastore", func(t *testing.T) {
		ds, err := NewTaskProgress(SQLite, ":memory:")
		require.NoError(t, err)
		require.NotNil(t, ds)

		err = ds.Close()
		require.NoError(t, err)
	})

	t.Run("Test PutProgress and GetProgress", func(t *testing.T) {
		ds, err := NewTaskProgress(SQLite, ":memory:")
		defer ds.Close()
		require.NoError(t, err)

		err = ds.PutProgress("task1", "progress1")
		require.NoError(t, err)

		progress, err := ds.GetProgress("task1")
		require.NoError(t, err)
		require.Equal(t, "progress1", progress)

		// Test get a non-exist task
		progress, err = ds.GetProgress("non_exist_task")
		require.NoError(t, err)
		require.Equal(t, "", progress)
	})

	t.Run("Test PutProgress with empty task id", func(t *testing.T) {
		ds, err := NewTaskProgress(SQLite, ":memory:")
		defer ds.Close()
		require.NoError(t, err)

		err = ds.PutProgress("", "progress1")
		require.Error(t, err)
	})
}
