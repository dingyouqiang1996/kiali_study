package store_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kiali/kiali/store"
)

func TestGetKeyExists(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	testStore.Replace(map[string]int{"key1": 42})
	value, err := testStore.Get("key1")

	require.NoError(err)
	require.Equal(42, value)
}

func TestGetNonExistantKeyFails(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	_, err := testStore.Get("nonexistent")
	require.Error(err)
	require.IsType(&store.NotFoundError{}, err)
}

func TestReplaceStoreContents(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	testStore.Replace(map[string]int{"key1": 42})

	newData := map[string]int{"key2": 99, "key3": 100}
	testStore.Replace(newData)

	_, err := testStore.Get("key1")
	require.Error(err)

	value, err := testStore.Get("key2")
	require.NoError(err)
	require.Equal(99, value)

	value, err = testStore.Get("key3")
	require.NoError(err)
	require.Equal(100, value)
}

func TestNotFoundImplementsStringer(t *testing.T) {
	require := require.New(t)

	err := &store.NotFoundError{Key: "key1"}
	require.NotEmpty(err.Error())
}

func TestReplaceWithEmptyKey(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	testStore.Replace(map[string]int{"": 1})

	val, err := testStore.Get("")
	require.NoError(err)
	require.Equal(1, val)
}

func TestReplaceIncrementsVersion(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	require.Equal(uint(0), testStore.Version())
	testStore.Replace(map[string]int{"": 1})

	require.Equal(uint(1), testStore.Version())
}

func TestSetNewKey(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	_, err := testStore.Get("key1")
	require.Error(err)

	currentVersion := testStore.Version()

	testStore.Set("key1", 42)
	val, err := testStore.Get("key1")
	require.NoError(err)
	require.Equal(42, val)
	require.Equal(currentVersion+1, testStore.Version())
}

func TestSetExistingKey(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	_, err := testStore.Get("key1")
	require.Error(err)

	testStore.Set("key1", 42)
	_, err = testStore.Get("key1")
	require.NoError(err)

	testStore.Set("key1", 43)
	val, err := testStore.Get("key1")
	require.NoError(err)
	require.Equal(43, val)
}

func TestItemsReturnsWholeMap(t *testing.T) {
	require := require.New(t)

	testStore := store.New[string, int]()
	testStore.Replace(map[string]int{"key1": 42, "key2": 99})

	contents := testStore.Items()
	require.Len(contents, 2)
	require.Equal(42, contents["key1"])
	require.Equal(99, contents["key2"])
}
