package main

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"

	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/decorators/cyber/intel"
	"github.com/MattermostFederal/mattermost-plugin-tactical-fusion/server/errcode"
)

const (
	directoryBundled    = "bundled"
	directoryConfigured = "configured"
)

type cyberDirectory struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type cyberDatasetFile struct {
	Name       string `json:"name"`
	Label      string `json:"label"`
	Path       string `json:"path"`
	Kind       string `json:"kind"`
	Size       int64  `json:"size"`
	Records    int    `json:"records"`
	CountError string `json:"countError"`
	Generated  string `json:"generated"`
}

type cyberDatabaseFile struct {
	Path  string `json:"path"`
	Kind  string `json:"kind"`
	Type  string `json:"type"`
	Built string `json:"built"`
	Size  int64  `json:"size"`
}

type cyberMissingDataset struct {
	Name  string `json:"name"`
	Label string `json:"label"`
}

type cyberSkippedFile struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

type cyberDatasetsResponse struct {
	Directories []cyberDirectory      `json:"directories"`
	Datasets    []cyberDatasetFile    `json:"datasets"`
	Databases   []cyberDatabaseFile   `json:"databases"`
	Missing     []cyberMissingDataset `json:"missing"`
	Replaced    []string              `json:"replaced"`
	Skipped     []cyberSkippedFile    `json:"skipped"`
}

func (p *Plugin) serveCyberDatasets(w http.ResponseWriter, r *http.Request, userID string) {
	if r.Method != http.MethodGet {
		writeAPIError(w, http.StatusMethodNotAllowed,
			errcode.WithCode(errcode.APIMethodNotAllowed, "Method not allowed."))
		return
	}
	if !p.API.HasPermissionTo(userID, model.PermissionManageSystem) {
		writeAPIError(w, http.StatusForbidden,
			errcode.WithCode(errcode.APINotAuthorized, "Not authorized."))
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeAPIJSON(w, http.StatusOK, p.cyberDatasetsBody())
}

func (p *Plugin) directoryKinds() (map[string]string, []cyberDirectory) {
	kinds := map[string]string{}
	var directories []cyberDirectory

	configured := strings.TrimSpace(p.getConfiguration().CyberDatasetsDir)
	for _, dir := range p.cyberDirs() {
		kind := directoryBundled
		if dir == configured {
			kind = directoryConfigured
		}
		kinds[filepath.Clean(dir)] = kind
		directories = append(directories, cyberDirectory{Path: dir, Kind: kind})
	}
	return kinds, directories
}

func kindOf(kinds map[string]string, path string) string {
	return kinds[filepath.Clean(filepath.Dir(path))]
}

func (p *Plugin) cyberDatasetsBody() cyberDatasetsResponse {
	kinds, directories := p.directoryKinds()
	inv := p.cyberIntel().Inventory()

	body := cyberDatasetsResponse{
		Directories: append([]cyberDirectory{}, directories...),
		Datasets:    []cyberDatasetFile{},
		Databases:   []cyberDatabaseFile{},
		Missing:     []cyberMissingDataset{},
		Replaced:    append([]string{}, inv.Replaced...),
		Skipped:     []cyberSkippedFile{},
	}

	loaded := map[string]bool{}
	for _, entry := range inv.Datasets {
		loaded[entry.Name] = true
		file := cyberDatasetFile{
			Name:      entry.Name,
			Label:     cyber.DatasetLabel(entry.Name),
			Path:      entry.Path,
			Kind:      kindOf(kinds, entry.Path),
			Size:      entry.Size,
			Records:   entry.Records,
			Generated: entry.Generated,
		}
		if entry.CountErr != nil {
			file.CountError = errcode.WithCode(errcode.CyberDataUnreadable, "The records could not be counted: "+entry.CountErr.Error())
		}
		body.Datasets = append(body.Datasets, file)
	}

	for _, name := range intel.Names {
		if !loaded[name] {
			body.Missing = append(body.Missing, cyberMissingDataset{Name: name, Label: cyber.DatasetLabel(name)})
		}
	}

	for _, database := range inv.Databases {
		built := ""
		if !database.Built.IsZero() {
			built = database.Built.Format("2006-01-02")
		}
		body.Databases = append(body.Databases, cyberDatabaseFile{
			Path:  database.Path,
			Kind:  kindOf(kinds, database.Path),
			Type:  database.Type,
			Built: built,
			Size:  database.Size,
		})
	}

	for _, problem := range inv.Problems {
		body.Skipped = append(body.Skipped, cyberSkippedFile{
			Path:   problem.Path,
			Reason: errcode.WithCode(cyberProblemCode(problem), cyberProblemMessage(problem)+": "+problem.Err.Error()),
		})
	}

	return body
}
