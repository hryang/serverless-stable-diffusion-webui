package datastore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSDServices(t *testing.T) {
	t.Run("Test NewSDServices", func(t *testing.T) {
		ds, err := NewSDServices(SQLite, ":memory:")
		require.NoError(t, err)
		require.NotNil(t, ds)

		err = ds.Close()
		require.NoError(t, err)
	})

	t.Run("Test PutServiceEndpoint and GetServiceEndpoint", func(t *testing.T) {
		sm, err := NewSDServices(SQLite, ":memory:")
		require.NoError(t, err)
		require.NotNil(t, sm)

		err = sm.PutServiceEndpoint("model1", "endpoint1")
		require.NoError(t, err)

		endpoint, err := sm.GetServiceEndpoint("model1")
		require.NoError(t, err)
		require.Equal(t, "endpoint1", endpoint)

		// Test get a non-exist model
		endpoint, err = sm.GetServiceEndpoint("non_exist_model")
		require.NoError(t, err)
		require.Equal(t, "", endpoint)

		// Test put with empty model name
		err = sm.PutServiceEndpoint("", "endpoint1")
		require.Error(t, err)
	})

	t.Run("Test ListAllServiceEndpoints", func(t *testing.T) {
		sds, err := NewSDServices(SQLite, ":memory:")
		assert.NoError(t, err)
		defer sds.Close()

		// Insert some test data.
		testData := map[string]string{
			"service1": "endpoint1",
			"service2": "endpoint2",
			"service3": "endpoint3",
		}
		for k, v := range testData {
			err = sds.PutServiceEndpoint(k, v)
			assert.NoError(t, err)
		}

		// Call ListAllServiceEndpoints and check the result.
		result, err := sds.ListAllServiceEndpoints()
		assert.NoError(t, err)
		assert.Equal(t, len(testData), len(result))
		for _, sd := range result {
			assert.Equal(t, testData[sd.Name], sd.Endpoint)
		}

		// Delete all data.
		for k := range testData {
			err = sds.ds.Delete(k)
			assert.NoError(t, err)
		}

		// Call ListAllServiceEndpoints again and check the result.
		result, err = sds.ListAllServiceEndpoints()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(result))
	})
}
