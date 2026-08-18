package model

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildUserListQueryNumericKeywordUsesTextArguments(t *testing.T) {
	// Given
	db := DB.Session(&gorm.Session{DryRun: true})

	// When
	query := buildUserListQuery(db, UserQueryParams{Keyword: "2175670275"})
	query.Find(&[]User{})

	// Then
	require.NotEmpty(t, query.Statement.Vars)
	for _, arg := range query.Statement.Vars {
		require.IsType(t, "", arg)
	}
}
