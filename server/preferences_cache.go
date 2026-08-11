package main

import (
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	// preferencesCacheSize bounds memory on a large server. Beyond it the
	// least recently used reader falls back to a KV read, which is the same
	// thing a cold node does.
	preferencesCacheSize = 1024

	// preferencesCacheTTL bounds how long a node can serve a stale blob if a
	// cluster invalidation is ever lost. Invalidation is the mechanism;
	// expiry is the backstop.
	preferencesCacheTTL = 10 * time.Minute

	// clusterEventInvalidatePreferences carries a user ID to the other nodes.
	// Without it a reader who saves on one node keeps seeing the old table on
	// every other node until the TTL runs out.
	clusterEventInvalidatePreferences = "cache_inv_prefs"
)

// cachingPreferenceStore reads through an in-memory cache and invalidates
// across the cluster on every write.
//
// The panel asks for these on every open and every hover, none of which is
// worth a KV read. Modelled on the caching store in mattermost-plugin-
// aocanywhere: read-through LRU with a TTL, and writes invalidate rather than
// repopulate so two nodes racing on the same key cannot leave one of them
// holding a value the other overwrote.
type cachingPreferenceStore struct {
	preferenceStore

	api   plugin.API
	cache *expirable.LRU[string, UserPreferences]
}

var _ preferenceStore = (*cachingPreferenceStore)(nil)

func newCachingPreferenceStore(inner preferenceStore, api plugin.API) *cachingPreferenceStore {
	return &cachingPreferenceStore{
		preferenceStore: inner,
		api:             api,
		cache:           expirable.NewLRU[string, UserPreferences](preferencesCacheSize, nil, preferencesCacheTTL),
	}
}

// Get serves from the cache when it can, and caches what it reads otherwise.
//
// The zero value is cached too. A reader who has never saved anything is the
// common case, and not caching them would mean the cache never helps the
// readers who need it most.
func (c *cachingPreferenceStore) Get(userID string) (UserPreferences, error) {
	if cached, ok := c.cache.Get(userID); ok {
		return cached.clone(), nil
	}

	prefs, err := c.preferenceStore.Get(userID)
	if err != nil {
		return UserPreferences{}, err
	}

	c.cache.Add(userID, prefs.clone())

	return prefs, nil
}

func (c *cachingPreferenceStore) Set(userID string, prefs UserPreferences) error {
	if err := c.preferenceStore.Set(userID, prefs); err != nil {
		return err
	}

	c.invalidate(userID)

	return nil
}

func (c *cachingPreferenceStore) Delete(userID string) error {
	if err := c.preferenceStore.Delete(userID); err != nil {
		return err
	}

	c.invalidate(userID)

	return nil
}

// HandleClusterEvent drops a key another node just wrote.
func (c *cachingPreferenceStore) HandleClusterEvent(ev model.PluginClusterEvent) {
	if ev.Id == clusterEventInvalidatePreferences {
		c.cache.Remove(string(ev.Data))
	}
}

// invalidate drops the key here and asks every other node to do the same.
//
// Best effort delivery: a lost event costs one reader a stale table until the
// TTL expires, which does not justify blocking a save on cluster traffic.
func (c *cachingPreferenceStore) invalidate(userID string) {
	c.cache.Remove(userID)

	if err := c.api.PublishPluginClusterEvent(model.PluginClusterEvent{
		Id:   clusterEventInvalidatePreferences,
		Data: []byte(userID),
	}, model.PluginClusterEventSendOptions{
		SendType: model.PluginClusterEventSendTypeBestEffort,
	}); err != nil {
		c.api.LogWarn("Failed to publish a preferences cache invalidation",
			"error_code", errcode.PreferencesCachePublishFailed, "user_id", userID, "error", err.Error())
	}
}
