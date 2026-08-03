package service

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/samber/hot"
)

const (
	channelAffinityForceCacheNamespace = "new-api:channel_affinity_force:v1"
	channelAffinityForceDuration       = 5 * time.Minute
	channelAffinityForceMinFenceTTL    = 24 * time.Hour
)

var (
	channelAffinityForceCacheOnce sync.Once
	channelAffinityForceCache     *cachex.HybridCache[string]
)

type channelAffinityForceRecord struct {
	ChannelID         int    `json:"channel_id"`
	ActivatedAtUnixNs int64  `json:"activated_at_unix_ns"`
	ForceUntilUnix    int64  `json:"force_until_unix"`
	CancelledAtUnix   int64  `json:"cancelled_at_unix,omitempty"`
	Group             string `json:"group"`
	Model             string `json:"model"`
}

func channelAffinityForceFenceTTL() time.Duration {
	setting := operation_setting.GetChannelAffinitySetting()
	maxSeconds := int64(channelAffinityForceMinFenceTTL / time.Second)
	if setting == nil {
		return channelAffinityForceMinFenceTTL
	}
	if int64(setting.DefaultTTLSeconds) > maxSeconds {
		maxSeconds = int64(setting.DefaultTTLSeconds)
	}
	for _, rule := range setting.Rules {
		if int64(rule.TTLSeconds) > maxSeconds {
			maxSeconds = int64(rule.TTLSeconds)
		}
	}
	return time.Duration(maxSeconds)*time.Second + channelAffinityForceDuration
}

func getChannelAffinityForceCache() *cachex.HybridCache[string] {
	channelAffinityForceCacheOnce.Do(func() {
		capacity := 100_000
		if setting := operation_setting.GetChannelAffinitySetting(); setting != nil && setting.MaxEntries > 0 {
			capacity = setting.MaxEntries
		}
		channelAffinityForceCache = cachex.NewHybridCache[string](cachex.HybridCacheConfig[string]{
			Namespace: cachex.Namespace(channelAffinityForceCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.StringCodec{},
			Memory: func() *hot.HotCache[string, string] {
				return hot.NewHotCache[string, string](hot.LRU, capacity).
					WithTTL(channelAffinityForceFenceTTL()).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityForceCache
}

func channelAffinityForceScopeKey(group, modelName string) string {
	raw := strings.TrimSpace(group) + "\x00" + strings.TrimSpace(modelName)
	return "scope:" + common.Sha1([]byte(raw))
}

func getChannelAffinityForceRecord(group, modelName string) (channelAffinityForceRecord, bool, error) {
	raw, found, err := getChannelAffinityForceCache().Get(channelAffinityForceScopeKey(group, modelName))
	if err != nil || !found {
		return channelAffinityForceRecord{}, false, err
	}
	var record channelAffinityForceRecord
	if err := common.UnmarshalJsonStr(raw, &record); err != nil {
		return channelAffinityForceRecord{}, false, fmt.Errorf("解析渠道亲和强制吸附状态失败: %w", err)
	}
	return record, true, nil
}

func setChannelAffinityForceRecord(record channelAffinityForceRecord) error {
	data, err := common.Marshal(record)
	if err != nil {
		return fmt.Errorf("编码渠道亲和强制吸附状态失败: %w", err)
	}
	return getChannelAffinityForceCache().SetWithTTL(
		channelAffinityForceScopeKey(record.Group, record.Model),
		string(data),
		channelAffinityForceFenceTTL(),
	)
}

func listChannelAffinityForceRecords() ([]channelAffinityForceRecord, error) {
	cache := getChannelAffinityForceCache()
	keys, err := cache.Keys()
	if err != nil {
		return nil, err
	}
	records := make([]channelAffinityForceRecord, 0, len(keys))
	for _, key := range keys {
		raw, found, err := cache.Get(key)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		var record channelAffinityForceRecord
		if err := common.UnmarshalJsonStr(raw, &record); err != nil {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}
