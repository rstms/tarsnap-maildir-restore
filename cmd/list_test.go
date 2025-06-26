package cmd

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCmdList(t *testing.T) {
	initTestConfig(t)
	archives, err := ListArchives()
	require.Nil(t, err)
	require.NotEmpty(t, archives)
}
