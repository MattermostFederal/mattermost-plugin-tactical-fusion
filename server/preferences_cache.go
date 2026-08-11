package main

import (
	"sync"
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

	// generation counts invalidations, so a read that started before one can
	// tell that it did and decline to cache what it found. Removing a key is
	// not enough on its own: a key still being read is not yet in the cache to
	// remove, so the write invalidates nothing and the slower read then
	// installs the value the write had just replaced.
	generationLock sync.Mutex
	generation     uint64
}

var _ preferenceStore = (*cachingPreferenceStore)(nil)

func newCachingPreferenceStore(inner preferenceStore, api plugin.API) *cachingPreferenceStore {
	return &cachingPreferenceStore{
		preferenceStore: inner,
		api:             api,
		cache:           expirable.NewLRU[string, UserPreferences](preferencesCacheSize, nil, preferencesCacheTTL),
	}
}

// currentGeneration reads the invalidation counter.
func (c *cachingPreferenceStore) currentGeneration() uint64 {
	c.generationLock.Lock()
	defer c.generationLock.Unlock()

	return c.generation
}

// bumpGeneration marks every read now in flight as stale.
func (c *cachingPreferenceStore) bumpGeneration() {
	c.generationLock.Lock()
	defer c.generationLock.Unlock()

	c.generation++
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

	// Read the generation before the store, so a write landing during the read
	// is guaranteed to change it.
	started := c.currentGeneration()

	prefs, err := c.preferenceStore.Get(userID)
	if err != nil {
		return UserPreferences{}, err
	}

	// Somebody wrote while this read was in flight, so what it holds is already
	// out of date. Return it, because it is what the store said when asked, but
	// do not install it: the next reader has to go and look again.
	if c.currentGeneration() == started {
		c.cache.Add(userID, prefs.clone())
	}

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
		// Bump first: a read in flight here is racing the other node's write
		// exactly as it would race a local one, and removing a key that is not
		// there yet would leave that read free to install the old value.
		c.bumpGeneration()
		c.cache.Remove(string(ev.Data))
	}
}

// invalidate drops the key here and asks every other node to do the same.
//
// Best effort delivery: a lost event costs one reader a stale table until the
// TTL expires, which does not justify blocking a save on cluster traffic.
func (c *cachingPreferenceStore) invalidate(userID string) {
	c.bumpGeneration()
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
