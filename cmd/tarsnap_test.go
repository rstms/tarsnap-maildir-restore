package cmd

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"testing"
)

func initTarsnap(t *testing.T) *Tarsnap {
	initTestConfig(t)
	ts, err := NewTarsnap(viper.GetString("archive"))
	require.Nil(t, err)
	require.NotNil(t, ts)
	err = ts.initialize()
	require.Nil(t, err)
	return ts
}

func checkTarsnap(t *testing.T, ts *Tarsnap) {
	require.NotEmpty(t, ts.Archive)
	require.NotEmpty(t, ts.Users)
	for username, user := range ts.Users {
		require.NotEmpty(t, username)
		require.NotEmpty(t, user.Maildirs)
		for _, maildir := range user.Maildirs {
			require.IsType(t, []MaildirFile{}, maildir.Files)
			require.NotEmpty(t, maildir.Files)
		}
	}
}

func TestTarsnapInit(t *testing.T) {
	viper.Set("user", "test")
	ts := initTarsnap(t)
	checkTarsnap(t, ts)
}

func TestTarsnapMetadataInit(t *testing.T) {
	viper.Set("user", "test")
	viper.Set("metadata_dir", "")
	ts := initTarsnap(t)
	checkTarsnap(t, ts)
}

func TestTarsnapRestore(t *testing.T) {
	viper.Set("user", "test")
	ts := initTarsnap(t)
	checkTarsnap(t, ts)
	err := ts.Restore()
	require.Nil(t, err)
}
