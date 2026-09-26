package main

import (
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

var bundledCyberDir = filepath.Join("assets", "cyber")

const (
	cyberCacheTTL    = 5 * time.Second
	cyberRetireGrace = 2 * time.Minute
)

type cyberDatasets struct {
	lock sync.Mutex

	set        *intel.Set
	checked    time.Time
	generation int
	refreshing bool
	opening    chan struct{}

	retireAfter time.Duration
}

func (p *Plugin) cyberIntel() *intel.Set {
	return p.cyberIntelFor(p.cyberDirs())
}

func (p *Plugin) cyberIntelFor(dirs []string) *intel.Set {
	for {
		p.cyber.lock.Lock()

		if set := p.cyber.set; set != nil {
			p.serveCachedCyberSetLocked(dirs)
			p.cyber.lock.Unlock()
			return set
		}

		if opening := p.cyber.opening; opening != nil {
			p.cyber.lock.Unlock()
			<-opening
			continue
		}

		opening := make(chan struct{})
		p.cyber.opening = opening
		generation := p.cyber.generation
		p.cyber.lock.Unlock()

		return p.openFirstCyberSet(dirs, opening, generation)
	}
}

func (p *Plugin) serveCachedCyberSetLocked(dirs []string) {
	if time.Since(p.cyber.checked) < cyberCacheTTL {
		return
	}

	if !p.cyber.set.Changed(dirs) {
		p.cyber.checked = time.Now()
		return
	}

	if !p.cyber.refreshing {
		p.cyber.refreshing = true
		go p.refreshCyberDatasets(dirs, p.cyber.generation)
	}
}

func (p *Plugin) openFirstCyberSet(dirs []string, opening chan struct{}, generation int) *intel.Set {
	var set *intel.Set

	defer func() {
		p.cyber.lock.Lock()
		p.cyber.opening = nil
		if set != nil {
			if p.cyber.generation == generation {
				p.cyber.set = set
				p.cyber.checked = time.Now()
			} else {
				p.retireCyberSetLocked(set)
			}
		}
		p.cyber.lock.Unlock()
		close(opening)
	}()

	set = p.openCyberDatasets(dirs)
	return set
}

func (p *Plugin) retireCyberSetLocked(set *intel.Set) {
	if set == nil {
		return
	}
	grace := p.cyber.retireAfter
	if grace == 0 {
		grace = cyberRetireGrace
	}
	time.AfterFunc(grace, set.Close)
}

func (p *Plugin) openCyberDatasets(dirs []string) *intel.Set {
	set, problems := intel.Open(dirs)
	for _, problem := range problems {
		p.warnOnce(problem.Path, cyberProblemMessage(problem),
			"error_code", cyberProblemCode(problem), "path", problem.Path, "error", problem.Err.Error())
	}
	return set
}

func (p *Plugin) refreshCyberDatasets(dirs []string, generation int) {
	set := p.openCyberDatasets(dirs)

	p.cyber.lock.Lock()
	defer p.cyber.lock.Unlock()

	if p.cyber.generation != generation {
		set.Close()
		return
	}
	p.cyber.refreshing = false
	p.retireCyberSetLocked(p.cyber.set)
	p.cyber.set = set
	p.cyber.checked = time.Now()
}

func (p *Plugin) reloadCyberDatasets() {
	p.cyber.lock.Lock()
	if p.cyber.set == nil {
		p.cyber.lock.Unlock()
		return
	}
	p.cyber.generation++
	generation := p.cyber.generation
	p.cyber.refreshing = true
	p.cyber.lock.Unlock()

	go p.refreshCyberDatasets(p.cyberDirs(), generation)
}

func (p *Plugin) warmCyberDatasets() {
	if _, err := p.API.GetBundlePath(); err != nil {
		return
	}
	dirs := p.cyberDirs()
	go p.cyberIntelFor(dirs)
}

func cyberProblemCode(problem *intel.FileError) int {
	switch problem.Class {
	case intel.ErrorSchema:
		return errcode.CyberDataSchemaMismatch
	case intel.ErrorName:
		return errcode.CyberDataBadName
	case intel.ErrorMMDB:
		return errcode.CyberDataMMDBUnreadable
	case intel.ErrorUnpack:
		return errcode.CyberDataUnpackFailed
	case intel.ErrorTooLarge:
		return errcode.CyberDataWatchlistTooLarge
	}

	return errcode.CyberDataUnreadable
}

func cyberProblemMessage(problem *intel.FileError) string {
	switch problem.Class {
	case intel.ErrorSchema:
		return "a cyber dataset carries no usable schema stamp and was skipped"
	case intel.ErrorName:
		return "a file in the cyber dataset directory is not a dataset this build reads"
	case intel.ErrorMMDB:
		return "a vendor IP database could not be read and was skipped"
	case intel.ErrorUnpack:
		return "a gzipped cyber dataset could not be unpacked beside itself"
	case intel.ErrorTooLarge:
		return "the cyber watchlist is larger than this build loads and was skipped"
	}

	return "a cyber dataset could not be read and was skipped"
}

func (p *Plugin) cyberDirs() []string {
	var dirs []string

	configured := strings.TrimSpace(p.getConfiguration().CyberDatasetsDir)

	if p.API == nil {
		if configured != "" {
			return []string{configured}
		}
		return nil
	}

	if bundle, err := p.API.GetBundlePath(); err == nil {
		dirs = append(dirs, filepath.Join(bundle, bundledCyberDir))
	} else {
		p.API.LogWarn("cannot locate the plugin bundle, so no bundled cyber datasets are read",
			"error_code", errcode.CyberDataNoBundlePath, "error", err.Error())
	}

	if configured != "" {
		dirs = append(dirs, configured)
	}

	return dirs
}

func (p *Plugin) forgetCyberDatasets() {
	p.cyber.lock.Lock()
	defer p.cyber.lock.Unlock()

	p.retireCyberSetLocked(p.cyber.set)
	p.cyber.set = nil
	p.cyber.checked = time.Time{}
	p.cyber.generation++
	p.cyber.refreshing = false
}
