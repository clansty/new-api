package service

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestShouldRecordChannelAffinityForce(t *testing.T) {
	activatedAt := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	forceUntil := activatedAt.Add(5 * time.Minute)
	record := channelAffinityForceRecord{
		ChannelID:         200,
		ActivatedAtUnixNs: activatedAt.UnixNano(),
		ForceUntilUnix:    forceUntil.Unix(),
		Group:             "default",
		Model:             "gpt-5",
	}

	tests := []struct {
		name           string
		meta           channelAffinityMeta
		finalChannelID int
		now            time.Time
		want           bool
	}{
		{
			name: "接管前开始的请求在窗口结束后仍不能回写旧渠道",
			meta: channelAffinityMeta{
				SelectedAtUnixNs: activatedAt.Add(-time.Second).UnixNano(),
			},
			finalChannelID: 100,
			now:            forceUntil.Add(time.Minute),
			want:           false,
		},
		{
			name: "接管请求回退成功后不能覆盖目标渠道亲和",
			meta: channelAffinityMeta{
				SelectedAtUnixNs: activatedAt.Add(time.Second).UnixNano(),
				ForceActivatedAt: activatedAt.UnixNano(),
				ForceChannelID:   200,
			},
			finalChannelID: 100,
			now:            forceUntil.Add(time.Minute),
			want:           false,
		},
		{
			name: "窗口结束后开始的新请求恢复正常写入",
			meta: channelAffinityMeta{
				SelectedAtUnixNs: forceUntil.Add(time.Second).UnixNano(),
			},
			finalChannelID: 100,
			now:            forceUntil.Add(2 * time.Second),
			want:           true,
		},
		{
			name: "接管被取消后恢复正常写入",
			meta: channelAffinityMeta{
				SelectedAtUnixNs: activatedAt.Add(-time.Second).UnixNano(),
			},
			finalChannelID: 100,
			now:            activatedAt.Add(2 * time.Minute),
			want:           true,
		},
		{
			name: "目标渠道成功时写入亲和",
			meta: channelAffinityMeta{
				SelectedAtUnixNs: activatedAt.Add(time.Second).UnixNano(),
				ForceActivatedAt: activatedAt.UnixNano(),
				ForceChannelID:   200,
			},
			finalChannelID: 200,
			now:            activatedAt.Add(2 * time.Minute),
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := record
			if tt.name == "接管被取消后恢复正常写入" {
				current.CancelledAtUnix = activatedAt.Add(time.Minute).Unix()
			}

			require.Equal(t, tt.want, shouldRecordChannelAffinityForce(current, tt.meta, tt.finalChannelID, tt.now))
		})
	}
}

func TestGetPreferredChannelByAffinity_whenForceActive(t *testing.T) {
	originalDB := model.DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	originalRDB := common.RDB
	t.Cleanup(func() {
		model.DB = originalDB
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		common.RDB = originalRDB
		channelAffinityForceCache = nil
		channelAffinityForceCacheOnce = sync.Once{}
	})

	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/affinity-force.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.RDB = nil
	channelAffinityForceCache = nil
	channelAffinityForceCacheOnce = sync.Once{}

	priority := int64(100)
	weight := uint(1)
	channel := model.Channel{
		Name:     "force-target",
		Status:   common.ChannelStatusEnabled,
		Models:   "gpt-5",
		Group:    "default",
		Priority: &priority,
		Weight:   &weight,
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-5",
		ChannelId: channel.Id,
		Enabled:   true,
		Priority:  &priority,
		Weight:    weight,
	}).Error)

	status, err := ActivateChannelAffinityForce(channel.Id)
	require.NoError(t, err)
	require.Equal(t, 1, status.ScopeCount)
	t.Cleanup(func() { _, _ = CancelChannelAffinityForce(channel.Id) })

	setting := operation_setting.GetChannelAffinitySetting()
	var rule operation_setting.ChannelAffinityRule
	for _, candidate := range setting.Rules {
		if candidate.Name == "codex cli trace" {
			rule = candidate
			break
		}
	}
	require.NotEmpty(t, rule.Name)
	affinityValue := fmt.Sprintf("force-%d", time.Now().UnixNano())
	cacheKeySuffix := buildChannelAffinityCacheKeySuffix(rule, "gpt-5", "default", affinityValue)
	require.NoError(t, getChannelAffinityCache().SetWithTTL(cacheKeySuffix, 999, time.Minute))
	t.Cleanup(func() { _, _ = getChannelAffinityCache().DeleteMany([]string{cacheKeySuffix}) })

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(fmt.Sprintf(`{"prompt_cache_key":%q}`, affinityValue)))
	context.Request.Header.Set("Content-Type", "application/json")

	channelID, found := GetPreferredChannelByAffinity(context, "gpt-5", "default")
	require.True(t, found)
	require.Equal(t, channel.Id, channelID)
	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(context))
}

func TestShouldSkipRetryAfterChannelAffinityFailure_whenForceActive(t *testing.T) {
	ctx := buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		SkipRetry:        true,
		ForceActivatedAt: time.Now().UnixNano(),
		ForceChannelID:   200,
	})
	ctx.Set(ginKeyChannelAffinitySkipRetry, true)

	require.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
}
