package main

import (
	"errors"
	"reflect"
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
