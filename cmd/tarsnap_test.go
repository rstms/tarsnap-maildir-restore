package cmd

import (
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTarsnapInit(t *testing.T) {
	initTestConfig(t)
	ts, err := NewTarsnap(viper.GetString("archive_name"))
	require.Nil(t, err)
	require.NotNil(t, ts)
	err = ts.initialize()
	require.Nil(t, err)
	require.IsType(t, []string{}, ts.Files)
	require.NotEmpty(t, ts.Files)
	require.NotEmpty(t, ts.Dirs)
}

func TestTarsnapMetadataInit(t *testing.T) {
	initTestConfig(t)
	viper.Set("file_list", "")
	viper.Set("dir_list", "")
	ts, err := NewTarsnap(viper.GetString("archive_name"))
	require.Nil(t, err)
	require.NotNil(t, ts)
	err = ts.initialize()
	require.Nil(t, err)
	require.IsType(t, []string{}, ts.Files)
	require.NotEmpty(t, ts.Files)
	require.NotEmpty(t, ts.Dirs)
}
