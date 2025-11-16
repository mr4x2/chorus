package s3

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/clyso/chorus/pkg/dom"
)

func TestStorageConfig_Validate(t *testing.T) {
	s := StorageConfig{
		Storages: map[string]Storage{
			"a": {Type: dom.StorageTypeDestination, Address: "a", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			"b": {Type: dom.StorageTypeSource, Address: "a", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			"c": {Type: dom.StorageTypeBoth, Address: "a", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			"d": {Type: dom.StorageTypeDestination, Address: "a", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
		},
	}
	r := require.New(t)
	r.NoError(s.Init())

	r.Equal([]string{"a", "b", "c", "d"}, s.storageList)
	r.Equal([]string{"b", "c"}, s.sourceList)
	r.Equal([]string{"a", "c", "d"}, s.destList)
	r.Equal("b", s.Main())
	r.Equal([]string{"a", "c", "d"}, s.Followers())
}

func TestStorageConfig_ValidateAddress(t *testing.T) {
	t.Run("Add http", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeBoth, Address: "clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			},
		}
		r.NoError(s.Init())
		r.EqualValues("http://clyso.com", s.Storages["a"].Address)
	})
	t.Run("Add https", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeBoth, IsSecure: true, Address: "clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			},
		}
		r.NoError(s.Init())
		r.EqualValues("https://clyso.com", s.Storages["a"].Address)
	})

	t.Run("Already http", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeBoth, Address: "http://clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			},
		}
		r.NoError(s.Init())
		r.EqualValues("http://clyso.com", s.Storages["a"].Address)
	})
	t.Run("Already https", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeBoth, IsSecure: true, Address: "https://clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
			},
		}
		r.NoError(s.Init())
		r.EqualValues("https://clyso.com", s.Storages["a"].Address)
	})

	t.Run("Invalid http", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeSource, Address: "https://clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
				"b": {Type: dom.StorageTypeDestination, Address: "http://dest", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"3", "4"}}},
			},
		}
		r.Error(s.Init())
	})
	t.Run("Invalid https", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeSource, IsSecure: true, Address: "http://clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
				"b": {Type: dom.StorageTypeDestination, Address: "http://dest", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"3", "4"}}},
			},
		}
		r.Error(s.Init())
	})
	t.Run("Invalid url", func(t *testing.T) {
		r := require.New(t)

		s := StorageConfig{
			Storages: map[string]Storage{
				"a": {Type: dom.StorageTypeSource, IsSecure: true, Address: "http:/clyso.com", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"1", "2"}}},
				"b": {Type: dom.StorageTypeDestination, Address: "http://dest", Provider: "p", Credentials: map[string]CredentialsV4{"user": {"3", "4"}}},
			},
		}
		r.Error(s.Init())
	})
}
