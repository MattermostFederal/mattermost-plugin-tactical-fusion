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
	// cluster invalidation is ever lost. Invalidation is the mechanism; expiry
	// is the backstop.
	//
	// Matched to the webapp's own cache lifetime in preferences/store.ts, so
	// the two layers in front of the KV store agree about how long a reader may
	// be shown something out of date. They are not the same kind of number: the
	// webapp has no invalidation to hear, so its timer is the only thing that
	// ever refreshes it, while this one is corrected by every write and only
	// matters for a write it never heard about. Aligning them is a deliberate
	// choice that this window is what a stale blob is worth, not an assumption
	// that they do the same job.
	//
	// The cost of the longer TTL is exactly one case: a cluster event dropped
	// between nodes leaves the other nodes serving the old blob for up to half
	// an hour rather than ten minutes. A reader on the node that saved sees
	// their change immediately either way.
	preferencesCacheTTL = 30 * time.Minute

	// clusterEventInvalidatePreferences carries a user ID to the other nodes.
	// Without it a reader who saves on one node keeps seeing the old table on
	// every other node until the TTL runs out.
	clusterEventInvalidatePreferences = "cache_inv_prefs"
)

// cachingPreferenceStore reads through an in-memory cache and invalidates
// across the cluster on every write.
//
// The panel asks for these on every open and every hover, none of which is
// worth a KV read. Modeled on the caching store in mattermost-plugin-
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
	//
	// The lock covers the fill as well as the counter, because checking and
	// filling separately leaves the same race in miniature: a read can pass the
	// check, a write can bump and find nothing to remove, and the read can then
	// install its value into the gap. That window is microseconds rather than a
	// KV round trip, which makes it rare enough to look correct under a test
	// and still reachable in production.
	//
	// One counter for every reader, not one per key. A write for somebody else
	// therefore makes this read decline to cache, which costs a KV read and
	// nothing else. The alternative is a per-key map with its own lifecycle,
	// and at any plausible rate of preference saves the coupling is not worth
	// it: the drop rate is the chance a write lands inside one KV read. It is
	// worth knowing that a degraded KV inverts that, since the window is the
	// read itself, so the cache stops filling exactly when it would help most.
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
//
// Called before the key is removed, never after. A reader that gets past the
// check has to find a Remove still ahead of it; bumping second would let one
// slip in behind the Remove with nothing left to undo it.
func (c *cachingPreferenceStore) bumpGeneration() {
	c.generationLock.Lock()
	defer c.generationLock.Unlock()

	c.generation++
}

// fillIfCurrent caches a value only if no invalidation has happened since the
// read that produced it began.
//
// The check and the write are one critical section on purpose. Splitting them
// is what the counter exists to prevent, only smaller.
func (c *cachingPreferenceStore) fillIfCurrent(userID string, startedAt uint64, prefs UserPreferences) {
	c.generationLock.Lock()
	defer c.generationLock.Unlock()

	if c.generation != startedAt {
		return
	}

	c.cache.Add(userID, prefs)
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

	// Somebody writing while this read was in flight means what it holds is
	// already out of date. It is still returned, because it is what the store
	// said when asked, but it is not installed: the next reader has to go and
	// look again. Cloned outside the lock to keep the section to a map write.
	c.fillIfCurrent(userID, started, prefs.clone())

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
