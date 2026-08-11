package main

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

// countingStore records how often the durable store was actually consulted,
// which is the only way to tell a cache hit from a very fast miss.
type countingStore struct {
	prefs   map[string]UserPreferences
	gets    int
	sets    int
	deletes int

	getErr    error
	setErr    error
	deleteErr error
}

func newCountingStore() *countingStore {
	return &countingStore{prefs: map[string]UserPreferences{}}
}

func (s *countingStore) Get(userID string) (UserPreferences, error) {
	s.gets++
	if s.getErr != nil {
		return UserPreferences{}, s.getErr
	}
	return s.prefs[userID].clone(), nil
}

func (s *countingStore) Set(userID string, prefs UserPreferences) error {
	s.sets++
	if s.setErr != nil {
		return s.setErr
	}
	s.prefs[userID] = prefs.clone()
	return nil
}

func (s *countingStore) Delete(userID string) error {
	s.deletes++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	delete(s.prefs, userID)
	return nil
}

// hookedStore runs a callback in the middle of a read, so a test can land a
// write inside the window between the store answering and the cache being
// filled.
type hookedStore struct {
	*countingStore

	duringGet func()
}

func (s *hookedStore) Get(userID string) (UserPreferences, error) {
	prefs, err := s.countingStore.Get(userID)
	if s.duringGet != nil {
		during := s.duringGet
		s.duringGet = nil
		during()
	}
	return prefs, err
}

// A save that lands while a read is in flight must not be undone by that read.
//
// Removing the key is not enough on its own: a key still being read is not in
// the cache yet, so there is nothing for the write to remove, and the slower
// read then installs the value the write had just replaced. The reader saves,
// the editor closes on the strength of it, and every later read serves the old
// table until the TTL runs out, which is indistinguishable from a save that
// silently did nothing.
func TestASaveDuringAReadIsNotOverwritten(t *testing.T) {
	inner := newCountingStore()
	inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 5}}

	api := newPreferenceAPI()
	store := newCachingPreferenceStore(&hookedStore{countingStore: inner}, api)

	// The write lands after the store has answered the read below, but before
	// that read reaches the cache.
	hooked := store.preferenceStore.(*hookedStore)
	hooked.duringGet = func() {
		if err := store.Set(testUserID, UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 99}}); err != nil {
			t.Errorf("Set() = %v", err)
		}
	}

	if _, err := store.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	// The next reader must see the saved value, not the one the in-flight read
	// was carrying.
	got, err := store.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if got.DTG.UrgentWithinMinutes != 99 {
		t.Fatalf("UrgentWithinMinutes = %d, want 99: the in-flight read overwrote the save",
			got.DTG.UrgentWithinMinutes)
	}
}

// The same race across nodes, which is the one the cluster event exists for.
func TestAClusterInvalidationDuringAReadIsNotOverwritten(t *testing.T) {
	inner := newCountingStore()
	inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 5}}

	api := newPreferenceAPI()
	store := newCachingPreferenceStore(&hookedStore{countingStore: inner}, api)

	hooked := store.preferenceStore.(*hookedStore)
	hooked.duringGet = func() {
		// Another node saved and told us to drop our copy. We have none yet.
		inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 99}}
		store.HandleClusterEvent(model.PluginClusterEvent{
			Id:   clusterEventInvalidatePreferences,
			Data: []byte(testUserID),
		})
	}

	if _, err := store.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	got, err := store.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if got.DTG.UrgentWithinMinutes != 99 {
		t.Fatalf("UrgentWithinMinutes = %d, want 99: the in-flight read outlived the invalidation",
			got.DTG.UrgentWithinMinutes)
	}
}

// lockedStore is a minimal thread-safe store, for the concurrent test below.
// countingStore is not safe to share between goroutines.
type lockedStore struct {
	mu    sync.Mutex
	prefs UserPreferences
}

func (s *lockedStore) Get(string) (UserPreferences, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.prefs.clone(), nil
}

func (s *lockedStore) Set(_ string, prefs UserPreferences) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prefs = prefs.clone()

	return nil
}

func (s *lockedStore) Delete(string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.prefs = UserPreferences{}

	return nil
}

// fillIfCurrent is the whole of the guarantee, so it is tested directly.
//
// The hook-based tests above reach the window between the store answering and
// the check. They cannot reach the one between the check and the fill, and no
// timing-based test reliably can: that window is a map write, and reproducing
// it took hundreds of thousands of attempts. It is closed structurally, by
// holding the lock across both, and pinned here by contract instead.
func TestFillIfCurrentDeclinesAStaleGeneration(t *testing.T) {
	store := newCachingPreferenceStore(newCountingStore(), newPreferenceAPI())

	started := store.currentGeneration()
	store.bumpGeneration()

	store.fillIfCurrent(testUserID, started, UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 5}})

	if _, ok := store.cache.Get(testUserID); ok {
		t.Fatal("fillIfCurrent cached a value read before an invalidation")
	}

	// And still fills when nothing moved, or the cache would never work.
	current := store.currentGeneration()
	store.fillIfCurrent(testUserID, current, UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 5}})

	if _, ok := store.cache.Get(testUserID); !ok {
		t.Fatal("fillIfCurrent declined a value read after the last invalidation")
	}
}

// A concurrency smoke test, run under -race by `make test`.
//
// It does NOT reliably reproduce the check-to-fill window; it exercises the
// concurrent path for the race detector and asserts the invariant on every
// round it does happen to interleave.
func TestNoStaleFillUnderConcurrency(t *testing.T) {
	const rounds = 2000

	// A large selection widens the clone, and with it the window between the
	// check and the fill.
	zones := make([]ZoneSelection, 0, maxZones)
	for j := range maxZones {
		zones = append(zones, ZoneSelection{IANA: "UTC", Name: string(rune('a' + (j % 26)))})
	}

	for i := range rounds {
		inner := &lockedStore{prefs: UserPreferences{DTG: DTGPreferences{
			UrgentWithinMinutes: 5,
			Zones:               zones,
		}}}

		store := newCachingPreferenceStore(inner, newPreferenceAPI())

		start := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			<-start
			_, _ = store.Get(testUserID)
		}()
		go func() {
			defer wg.Done()
			<-start
			_ = store.Set(testUserID, UserPreferences{DTG: DTGPreferences{UrgentWithinMinutes: 99}})
		}()

		close(start)
		wg.Wait()

		// Set finished, so its Remove has run. Anything cached now was put
		// there by the read, and the read carried the pre-save value.
		if cached, ok := store.cache.Get(testUserID); ok && cached.DTG.UrgentWithinMinutes != 99 {
			t.Fatalf("round %d: the read installed the pre-save value after the invalidation "+
				"(UrgentWithinMinutes = %d, want 99 or nothing cached)",
				i, cached.DTG.UrgentWithinMinutes)
		}
	}
}

func TestCacheServesTheSecondRead(t *testing.T) {
	inner := newCountingStore()
	inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "UTC"}}}}
	cache := newCachingPreferenceStore(inner, newPreferenceAPI())

	first, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	second, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}

	if inner.gets != 1 {
		t.Fatalf("durable reads = %d, want 1", inner.gets)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("cached read differs: %+v vs %+v", first, second)
	}
}

// The reader who has never saved anything is the common case. Not caching them
// would mean the cache never helps the readers who need it most.
func TestCacheCachesTheDefaults(t *testing.T) {
	inner := newCountingStore()
	cache := newCachingPreferenceStore(inner, newPreferenceAPI())

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	if inner.gets != 1 {
		t.Fatalf("durable reads = %d, want 1", inner.gets)
	}
}

func TestCacheDoesNotCacheAFailedRead(t *testing.T) {
	inner := newCountingStore()
	inner.getErr = errors.New("boom")
	cache := newCachingPreferenceStore(inner, newPreferenceAPI())

	if _, err := cache.Get(testUserID); err == nil {
		t.Fatal("Get() succeeded despite a failing store")
	}

	inner.getErr = nil
	inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "UTC"}}}}

	got, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if len(got.DTG.Zones) != 1 {
		t.Fatalf("Get() = %+v, want the value the store now holds", got)
	}
}

// A caller that edits what it was handed must not be editing the cache.
func TestCacheHandsOutCopies(t *testing.T) {
	inner := newCountingStore()
	inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "UTC"}}}}
	cache := newCachingPreferenceStore(inner, newPreferenceAPI())

	first, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	first.DTG.Zones[0] = ZoneSelection{IANA: "Asia/Tokyo"}

	second, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if second.DTG.Zones[0].IANA != "UTC" {
		t.Fatalf("the cache was mutated through a returned value: %v", second.DTG.Zones)
	}
}

func TestSaveInvalidatesAndTellsTheCluster(t *testing.T) {
	inner := newCountingStore()
	api := newPreferenceAPI()
	cache := newCachingPreferenceStore(inner, api)

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	saved := UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "Asia/Tokyo"}}}}
	if err := cache.Set(testUserID, saved); err != nil {
		t.Fatalf("Set() = %v", err)
	}

	got, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if !reflect.DeepEqual(got.DTG.Zones, []ZoneSelection{{IANA: "Asia/Tokyo"}}) {
		t.Fatalf("Get() after Set = %+v, want the saved value", got)
	}
	if inner.gets != 2 {
		t.Fatalf("durable reads = %d, want 2 (the save must drop the cached copy)", inner.gets)
	}

	if len(api.published) != 1 {
		t.Fatalf("published %d cluster events, want 1", len(api.published))
	}
	if api.published[0].Id != clusterEventInvalidatePreferences {
		t.Fatalf("event id = %q, want %q", api.published[0].Id, clusterEventInvalidatePreferences)
	}
	if string(api.published[0].Data) != testUserID {
		t.Fatalf("event data = %q, want the user id", api.published[0].Data)
	}
}

func TestDeleteInvalidatesAndTellsTheCluster(t *testing.T) {
	inner := newCountingStore()
	inner.prefs[testUserID] = UserPreferences{DTG: DTGPreferences{Zones: []ZoneSelection{{IANA: "Asia/Tokyo"}}}}
	api := newPreferenceAPI()
	cache := newCachingPreferenceStore(inner, api)

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if err := cache.Delete(testUserID); err != nil {
		t.Fatalf("Delete() = %v", err)
	}

	got, err := cache.Get(testUserID)
	if err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if len(got.DTG.Zones) != 0 {
		t.Fatalf("Get() after Delete = %+v, want the defaults", got)
	}
	if len(api.published) != 1 {
		t.Fatalf("published %d cluster events, want 1", len(api.published))
	}
}

// A save that never reached the store must not drop a cached copy that is
// still correct, and must not tell the cluster that anything changed.
func TestAFailedSaveDoesNotInvalidate(t *testing.T) {
	inner := newCountingStore()
	inner.setErr = errors.New("boom")
	api := newPreferenceAPI()
	cache := newCachingPreferenceStore(inner, api)

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if err := cache.Set(testUserID, UserPreferences{}); err == nil {
		t.Fatal("Set() succeeded despite a failing store")
	}

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if inner.gets != 1 {
		t.Fatalf("durable reads = %d, want 1 (a failed save must leave the cache alone)", inner.gets)
	}
	if len(api.published) != 0 {
		t.Fatalf("published %d cluster events, want 0", len(api.published))
	}
}

func TestClusterEventDropsTheCachedCopy(t *testing.T) {
	inner := newCountingStore()
	api := newPreferenceAPI()
	cache := newCachingPreferenceStore(inner, api)

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	cache.HandleClusterEvent(model.PluginClusterEvent{
		Id:   clusterEventInvalidatePreferences,
		Data: []byte(testUserID),
	})

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if inner.gets != 2 {
		t.Fatalf("durable reads = %d, want 2", inner.gets)
	}
}

func TestClusterEventIgnoresOtherEvents(t *testing.T) {
	inner := newCountingStore()
	cache := newCachingPreferenceStore(inner, newPreferenceAPI())

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	cache.HandleClusterEvent(model.PluginClusterEvent{
		Id:   "something_else",
		Data: []byte(testUserID),
	})

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if inner.gets != 1 {
		t.Fatalf("durable reads = %d, want 1", inner.gets)
	}
}

// Cluster delivery is best effort. A save that could not be broadcast still
// saved, and must report success.
func TestSaveSucceedsWhenTheBroadcastFails(t *testing.T) {
	inner := newCountingStore()
	api := newPreferenceAPI()
	api.publishErr = errors.New("no cluster")
	cache := newCachingPreferenceStore(inner, api)

	if err := cache.Set(testUserID, UserPreferences{}); err != nil {
		t.Fatalf("Set() = %v, want nil", err)
	}
	if len(api.warnings) == 0 {
		t.Fatal("a failed broadcast was swallowed without a warning")
	}
}

// The mirror of TestAFailedSaveDoesNotInvalidate, for the delete side. A delete
// that failed has changed nothing, so dropping the cached copy would send every
// node back to the store for a value that is still exactly what they hold.
func TestAFailedDeleteDoesNotInvalidate(t *testing.T) {
	inner := newCountingStore()
	inner.deleteErr = errors.New("boom")
	api := newPreferenceAPI()
	cache := newCachingPreferenceStore(inner, api)

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}

	err := cache.Delete(testUserID)
	if err == nil {
		t.Fatal("Delete() succeeded despite a failing store")
	}
	if !errors.Is(err, inner.deleteErr) {
		t.Fatalf("Delete() = %v, which does not carry the store's failure", err)
	}

	if _, err := cache.Get(testUserID); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if inner.gets != 1 {
		t.Fatalf("durable reads = %d, want 1 (a failed delete must leave the cache alone)", inner.gets)
	}
	if len(api.published) != 0 {
		t.Fatalf("published %d cluster events, want 0", len(api.published))
	}
}
