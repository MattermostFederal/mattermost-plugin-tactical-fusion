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

const cyberCacheTTL = 5 * time.Second

type cyberDatasets struct {
	lock sync.Mutex

	set     *intel.Set
	checked time.Time
}

func (p *Plugin) cyberIntel() *intel.Set {
	dirs := p.cyberDirs()

	p.cyber.lock.Lock()
	defer p.cyber.lock.Unlock()

	if p.cyber.set != nil && time.Since(p.cyber.checked) < cyberCacheTTL {
		return p.cyber.set
	}

	if p.cyber.set != nil && !p.cyber.set.Changed(dirs) {
		p.cyber.checked = time.Now()
		return p.cyber.set
	}

	set, problems := intel.Open(dirs)
	for _, problem := range problems {
		p.warnOnce(problem.Path, cyberProblemMessage(problem),
			"error_code", cyberProblemCode(problem), "path", problem.Path, "error", problem.Err.Error())
	}

	p.cyber.set = set
	p.cyber.checked = time.Now()

	return set
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

	p.cyber.set = nil
	p.cyber.checked = time.Time{}
}
