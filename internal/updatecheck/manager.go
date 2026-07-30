package updatecheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRefreshInterval = 24 * time.Hour
	defaultReleasesURL     = "https://api.github.com/repos/pdparchitect/launcher/releases?per_page=100"
	maxReleaseResponse     = 2 << 20
)

var (
	versionPattern = regexp.MustCompile(
		`^([0-9]+)\.([0-9]+)\.([0-9]+)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`,
	)
	stableTagPattern = regexp.MustCompile(
		`^v([0-9]+)\.([0-9]+)\.([0-9]+)$`,
	)
)

type Status struct {
	CurrentVersion  string     `json:"currentVersion"`
	LatestVersion   string     `json:"latestVersion,omitempty"`
	ReleaseURL      string     `json:"releaseUrl,omitempty"`
	UpdateAvailable bool       `json:"updateAvailable"`
	Checking        bool       `json:"checking"`
	CheckedAt       *time.Time `json:"checkedAt,omitempty"`
}

type Options struct {
	Client          *http.Client
	ReleasesURL     string
	RefreshInterval time.Duration
	Now             func() time.Time
}

type cacheState struct {
	CheckedAt     time.Time `json:"checkedAt"`
	ETag          string    `json:"etag,omitempty"`
	LatestVersion string    `json:"latestVersion,omitempty"`
	ReleaseURL    string    `json:"releaseUrl,omitempty"`
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	HTMLURL    string `json:"html_url"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type semanticVersion struct {
	numbers    [3]uint64
	prerelease bool
}

type Manager struct {
	mutex           sync.RWMutex
	refreshMutex    sync.Mutex
	root            string
	currentVersion  string
	current         semanticVersion
	enabled         bool
	client          *http.Client
	releasesURL     string
	refreshInterval time.Duration
	now             func() time.Time
	state           cacheState
	checking        bool
}

func NewManager(root string, currentVersion string, options Options) *Manager {
	if options.Client == nil {
		options.Client = &http.Client{Timeout: 15 * time.Second}
	}
	if options.ReleasesURL == "" {
		options.ReleasesURL = defaultReleasesURL
	}
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = DefaultRefreshInterval
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	current, enabled := parseVersion(currentVersion)
	manager := &Manager{
		root:            filepath.Join(root, "updates"),
		currentVersion:  currentVersion,
		current:         current,
		enabled:         enabled,
		client:          options.Client,
		releasesURL:     options.ReleasesURL,
		refreshInterval: options.RefreshInterval,
		now:             options.Now,
	}
	manager.restore()
	return manager
}

func (manager *Manager) Status() Status {
	manager.mutex.RLock()
	defer manager.mutex.RUnlock()
	return manager.statusLocked()
}

func (manager *Manager) Refresh(
	ctx context.Context,
	force bool,
) (bool, error) {
	if !manager.enabled {
		return false, nil
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	manager.refreshMutex.Lock()
	defer manager.refreshMutex.Unlock()

	manager.mutex.RLock()
	state := manager.state
	manager.mutex.RUnlock()
	now := manager.now().UTC()
	if !force && !state.CheckedAt.IsZero() &&
		now.Sub(state.CheckedAt) < manager.refreshInterval {
		return manager.updateAvailable(state.LatestVersion), nil
	}

	manager.setChecking(true)
	defer manager.setChecking(false)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		manager.releasesURL,
		nil,
	)
	if err != nil {
		return false, fmt.Errorf("create Launcher update request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2026-03-10")
	request.Header.Set("User-Agent", "pdparchitect-launcher")
	if state.ETag != "" {
		request.Header.Set("If-None-Match", state.ETag)
	}

	response, err := manager.client.Do(request)
	if err != nil {
		return false, fmt.Errorf("check Launcher releases: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotModified {
		state.CheckedAt = now
		if err := manager.commitState(state); err != nil {
			return false, err
		}
		return manager.updateAvailable(state.LatestVersion), nil
	}
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, response.Body)
		return false, fmt.Errorf(
			"check Launcher releases: GitHub returned %s",
			response.Status,
		)
	}

	data, err := readBounded(response.Body, maxReleaseResponse)
	if err != nil {
		return false, fmt.Errorf("read Launcher releases: %w", err)
	}
	var releases []githubRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return false, fmt.Errorf("decode Launcher releases: %w", err)
	}
	release, latest, err := selectRelease(releases)
	if err != nil {
		return false, err
	}
	state = cacheState{
		CheckedAt:     now,
		ETag:          response.Header.Get("ETag"),
		LatestVersion: latest,
		ReleaseURL:    release.HTMLURL,
	}
	if err := manager.commitState(state); err != nil {
		return false, err
	}
	return manager.updateAvailable(latest), nil
}

func (manager *Manager) statusLocked() Status {
	var checkedAt *time.Time
	if !manager.state.CheckedAt.IsZero() {
		value := manager.state.CheckedAt
		checkedAt = &value
	}
	return Status{
		CurrentVersion:  manager.currentVersion,
		LatestVersion:   manager.state.LatestVersion,
		ReleaseURL:      manager.state.ReleaseURL,
		UpdateAvailable: manager.updateAvailable(manager.state.LatestVersion),
		Checking:        manager.checking,
		CheckedAt:       checkedAt,
	}
}

func (manager *Manager) updateAvailable(latest string) bool {
	version, valid := parseVersion(latest)
	return manager.enabled && valid && compareVersion(version, manager.current) > 0
}

func (manager *Manager) setChecking(checking bool) {
	manager.mutex.Lock()
	manager.checking = checking
	manager.mutex.Unlock()
}

func (manager *Manager) restore() {
	data, err := os.ReadFile(manager.statePath())
	if err != nil {
		return
	}
	var state cacheState
	if json.Unmarshal(data, &state) != nil {
		return
	}
	if state.LatestVersion != "" {
		if _, valid := parseStableVersion(state.LatestVersion); !valid ||
			!validReleaseURL(state.ReleaseURL) {
			return
		}
	}
	manager.state = state
}

func (manager *Manager) commitState(state cacheState) error {
	if err := writeJSONAtomic(manager.statePath(), state); err != nil {
		return fmt.Errorf("save Launcher update state: %w", err)
	}
	manager.mutex.Lock()
	manager.state = state
	manager.mutex.Unlock()
	return nil
}

func (manager *Manager) statePath() string {
	return filepath.Join(manager.root, "state.json")
}

func selectRelease(
	releases []githubRelease,
) (githubRelease, string, error) {
	var selected githubRelease
	var selectedVersion semanticVersion
	var selectedText string
	found := false
	for _, release := range releases {
		if release.Draft || release.Prerelease ||
			!validReleaseURL(release.HTMLURL) {
			continue
		}
		versionText, version, valid := versionFromTag(release.TagName)
		if !valid || found && compareVersion(version, selectedVersion) <= 0 {
			continue
		}
		selected = release
		selectedVersion = version
		selectedText = versionText
		found = true
	}
	if !found {
		return githubRelease{}, "", errors.New(
			"no published stable Launcher release was found",
		)
	}
	return selected, selectedText, nil
}

func versionFromTag(tag string) (string, semanticVersion, bool) {
	matches := stableTagPattern.FindStringSubmatch(tag)
	if matches == nil {
		return "", semanticVersion{}, false
	}
	text := strings.TrimPrefix(tag, "v")
	version, valid := parseStableVersion(text)
	return text, version, valid
}

func parseStableVersion(value string) (semanticVersion, bool) {
	version, valid := parseVersion(value)
	return version, valid && !version.prerelease
}

func parseVersion(value string) (semanticVersion, bool) {
	matches := versionPattern.FindStringSubmatch(value)
	if matches == nil {
		return semanticVersion{}, false
	}
	var version semanticVersion
	for index := range version.numbers {
		number, err := strconv.ParseUint(matches[index+1], 10, 64)
		if err != nil {
			return semanticVersion{}, false
		}
		version.numbers[index] = number
	}
	version.prerelease = strings.Contains(value, "-")
	return version, true
}

func compareVersion(left semanticVersion, right semanticVersion) int {
	for index := range left.numbers {
		switch {
		case left.numbers[index] < right.numbers[index]:
			return -1
		case left.numbers[index] > right.numbers[index]:
			return 1
		}
	}
	switch {
	case left.prerelease && !right.prerelease:
		return -1
	case !left.prerelease && right.prerelease:
		return 1
	default:
		return 0
	}
}

func validReleaseURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil &&
		parsed.Scheme == "https" &&
		parsed.Host == "github.com" &&
		strings.HasPrefix(
			parsed.Path,
			"/pdparchitect/launcher/releases/",
		)
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d-byte limit", limit)
	}
	return data, nil
}

func writeJSONAtomic(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(name, append(data, '\n'), 0o600)
}

func writeFileAtomic(name string, data []byte, mode fs.FileMode) error {
	directory := filepath.Dir(name)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".update-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, name); err != nil {
		return err
	}
	committed = true
	return nil
}
