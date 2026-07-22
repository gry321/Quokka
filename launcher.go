package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
	"syscall"

	"golang.org/x/sys/windows/registry"

	"github.com/mozillazg/go-pinyin"
)

// AppEntry represents a launchable application.
type AppEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Icon string `json:"icon"` // populated lazily via GetAppIcon
}

// LauncherIndex caches all indexed applications.
type LauncherIndex struct {
	entries []AppEntry
}

// NewLauncherIndex scans multiple sources and builds a deduplicated index.
func NewLauncherIndex() *LauncherIndex {
	li := &LauncherIndex{}

	type result struct {
		entries []AppEntry
	}

	// Run all scanners in parallel
	var (
		startMenuCh  = make(chan result, 1)
		registryCh   = make(chan result, 1)
		directoryCh  = make(chan result, 1)
		desktopLnkCh = make(chan result, 1)
	)

	go func() { startMenuCh <- result{li.scanStartMenu()} }()
	go func() { registryCh <- result{li.scanRegistryUninstall()} }()
	go func() { directoryCh <- result{li.scanDirectories()} }()
	go func() { desktopLnkCh <- result{li.scanDesktopAndLnk()} }()

	seen := make(map[string]bool)
	var all []AppEntry

	addEntries := func(entries []AppEntry) {
		for _, e := range entries {
			key := strings.ToLower(strings.TrimSpace(e.Path))
			if key == "" || seen[key] {
				continue
			}
			// Skip obvious non-app items
			if isJunkEntry(e) {
				continue
			}
			seen[key] = true
			all = append(all, e)
		}
	}

	addEntries((<-startMenuCh).entries)
	addEntries((<-registryCh).entries)
	addEntries((<-directoryCh).entries)
	addEntries((<-desktopLnkCh).entries)

	// Sort alphabetically by name
	sort.Slice(all, func(i, j int) bool {
		return strings.ToLower(all[i].Name) < strings.ToLower(all[j].Name)
	})

	li.entries = all
	return li
}

// isJunkEntry filters out non-launchable items.
func isJunkEntry(e AppEntry) bool {
	name := strings.ToLower(e.Name)
	path := strings.ToLower(e.Path)
	// Skip uninstallers, help files, etc.
	junkSubstrings := []string{
		"uninstall", "remove", "help", "documentation",
		"readme", "license", "release notes", "setup.",
	}
	for _, junk := range junkSubstrings {
		if strings.Contains(name, junk) {
			return true
		}
	}
	// Only allow .exe, .bat, .cmd, .msc — exclude .lnk (icon has arrow overlay)
	ext := filepath.Ext(path)
	switch strings.ToLower(ext) {
	case ".exe", ".bat", ".cmd", ".msc":
		return false
	}
	return true
}

// ============================================================
// Source 1: Start Menu (existing — unchanged)
// ============================================================

const psResolveScript = `$dirs = @($env:APPDATA + '\Microsoft\Windows\Start Menu\Programs', 'C:\ProgramData\Microsoft\Windows\Start Menu\Programs')
$shell = New-Object -ComObject WScript.Shell
$results = @()
foreach ($dir in $dirs) {
    if (Test-Path $dir) {
        Get-ChildItem $dir -Recurse -Include *.lnk | ForEach-Object {
            try {
                $sc = $shell.CreateShortcut($_.FullName)
                if ($sc.TargetPath) {
                    $results += [PSCustomObject]@{n=$_.BaseName; p=$sc.TargetPath}
                }
            } catch {}
        }
    }
}
$results | ConvertTo-Json -Compress`

func (li *LauncherIndex) scanStartMenu() []AppEntry {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", psResolveScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	out, err := cmd.Output()
	if err != nil {
		return li.fallbackScan()
	}
	return li.parsePSOutput(out)
}

func (li *LauncherIndex) parsePSOutput(out []byte) []AppEntry {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return li.fallbackScan()
	}
	var entries []AppEntry
	if out[0] == '[' {
		var items []struct {
			N string `json:"n"`
			P string `json:"p"`
		}
		if json.Unmarshal(out, &items) == nil {
			for _, it := range items {
				if it.N != "" && it.P != "" {
					entries = append(entries, AppEntry{Name: it.N, Path: it.P})
				}
			}
		}
	} else {
		var item struct {
			N string `json:"n"`
			P string `json:"p"`
		}
		if json.Unmarshal(out, &item) == nil && item.N != "" && item.P != "" {
			entries = append(entries, AppEntry{Name: item.N, Path: item.P})
		}
	}
	if len(entries) == 0 {
		return li.fallbackScan()
	}
	return entries
}

func (li *LauncherIndex) fallbackScan() []AppEntry {
	// .lnk files are excluded globally (icon has arrow overlay).
	// The PowerShell scan resolves .lnk → target .exe; fallback returns nothing.
	return nil
}

// ============================================================
// Source 2: Registry Uninstall Keys (installed programs)
// ============================================================

func (li *LauncherIndex) scanRegistryUninstall() []AppEntry {
	regPaths := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall`,
	}

	var mu sync.Mutex
	var entries []AppEntry
	var wg sync.WaitGroup

	// Scan HKLM branches in parallel
	for _, rp := range regPaths {
		wg.Add(1)
		go func(regPath string) {
			defer wg.Done()
			found := readUninstallBranch(registry.LOCAL_MACHINE, regPath)
			mu.Lock()
			entries = append(entries, found...)
			mu.Unlock()
		}(rp)
	}

	// Scan HKCU branch
	wg.Add(1)
	go func() {
		defer wg.Done()
		found := readUninstallBranch(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`)
		mu.Lock()
		entries = append(entries, found...)
		mu.Unlock()
	}()

	wg.Wait()
	return entries
}

func readUninstallBranch(root registry.Key, path string) []AppEntry {
	k, err := registry.OpenKey(root, path, registry.READ|registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}

	var entries []AppEntry
	for _, subkey := range subkeys {
		sk, err := registry.OpenKey(root, path+`\`+subkey, registry.READ)
		if err != nil {
			continue
		}

		displayName, _, err1 := sk.GetStringValue("DisplayName")
		installLocation, _, _ := sk.GetStringValue("InstallLocation")
		displayIcon, _, _ := sk.GetStringValue("DisplayIcon")
		uninstallString, _, _ := sk.GetStringValue("UninstallString")

		sk.Close()

		if err1 != nil || displayName == "" {
			continue
		}

		// Try to find the main executable path
		exePath := resolveExePath(displayIcon, installLocation, uninstallString)
		if exePath == "" {
			continue
		}

		entries = append(entries, AppEntry{
			Name: displayName,
			Path: exePath,
		})
	}
	return entries
}

// resolveExePath attempts to determine the main executable path from registry fields.
func resolveExePath(displayIcon, installLocation, uninstallString string) string {
	// 1. DisplayIcon often contains "path\to\app.exe,0"
	if displayIcon != "" {
		p := displayIcon
		if idx := strings.LastIndex(p, ","); idx != -1 {
			p = p[:idx]
		}
		p = strings.Trim(p, ` "`)
		if strings.HasSuffix(strings.ToLower(p), ".exe") {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}

	// 2. InstallLocation + look for .exe
	if installLocation != "" {
		installLocation = strings.Trim(installLocation, ` "`)
		if exe := findMainExe(installLocation); exe != "" {
			return exe
		}
	}

	// 3. Parse uninstall string for exe path
	if uninstallString != "" {
		uninstallString = strings.TrimSpace(uninstallString)
		// Handle quoted paths: "C:\path\to\uninstall.exe" /args
		if strings.HasPrefix(uninstallString, `"`) {
			end := strings.Index(uninstallString[1:], `"`)
			if end != -1 {
				p := uninstallString[1 : end+1]
				if strings.HasSuffix(strings.ToLower(p), ".exe") {
					if _, err := os.Stat(p); err == nil {
						// Use the uninstaller's directory to find the main exe
						dir := filepath.Dir(p)
						if exe := findMainExe(dir); exe != "" && exe != p {
							return exe
						}
						return p
					}
				}
			}
		}
	}

	return ""
}

// findMainExe looks for the most likely main executable in a directory.
func findMainExe(dir string) string {
	dir = strings.Trim(dir, ` "`)
	if dir == "" {
		return ""
	}

	// First check if dir itself is an exe file
	if strings.HasSuffix(strings.ToLower(dir), ".exe") {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
		return ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	dirName := strings.ToLower(filepath.Base(dir))
	var best string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
			continue
		}
		name := strings.ToLower(e.Name())
		// Skip obvious non-main executables
		if strings.Contains(name, "uninstall") || strings.Contains(name, "setup") ||
			strings.Contains(name, "helper") || strings.Contains(name, "updater") {
			continue
		}
		// Prefer exe whose name matches the directory name
		exeBase := strings.TrimSuffix(name, ".exe")
		if strings.Contains(strings.ToLower(dirName), exeBase) || strings.Contains(exeBase, dirName) {
			return filepath.Join(dir, e.Name())
		}
		if best == "" {
			best = filepath.Join(dir, e.Name())
		}
	}
	return best
}

// ============================================================
// Source 3: Directory scan (Program Files + AppData + ProgramData)
// ============================================================

func (li *LauncherIndex) scanDirectories() []AppEntry {
	dirs := []string{
		`C:\Program Files`,
		`C:\Program Files (x86)`,
		os.Getenv("APPDATA"),
		os.Getenv("LOCALAPPDATA"),
		os.Getenv("ProgramData"),
	}

	var mu sync.Mutex
	var entries []AppEntry
	var wg sync.WaitGroup

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			found := scanDirForExes(d, 3) // max depth 3
			mu.Lock()
			entries = append(entries, found...)
			mu.Unlock()
		}(dir)
	}

	wg.Wait()
	return entries
}

func scanDirForExes(root string, maxDepth int) []AppEntry {
	var entries []AppEntry
	rootDepth := strings.Count(root, string(os.PathSeparator))

	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		depth := strings.Count(path, string(os.PathSeparator)) - rootDepth
		if depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			// Skip common non-app directories
			name := strings.ToLower(filepath.Base(path))
			if name == "node_modules" || name == ".git" || name == "cache" || name == "logs" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(info.Name()), ".exe") {
			n := strings.TrimSuffix(info.Name(), ".exe")
			entries = append(entries, AppEntry{Name: n, Path: path})
		}
		return nil
	})
	return entries
}

// ============================================================
// Source 4: Desktop shortcuts and other .lnk files
// ============================================================

func (li *LauncherIndex) scanDesktopAndLnk() []AppEntry {
	dirs := []string{
		filepath.Join(os.Getenv("USERPROFILE"), "Desktop"),
		filepath.Join(os.Getenv("PUBLIC"), "Desktop"),
	}

	// Use PowerShell to resolve .lnk targets on Desktop
	script := `$shell = New-Object -ComObject WScript.Shell
$results = @()
`
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		script += `
$dir = '` + dir + `'
if (Test-Path $dir) {
    Get-ChildItem $dir -Filter *.lnk | ForEach-Object {
        try {
            $sc = $shell.CreateShortcut($_.FullName)
            if ($sc.TargetPath -and $sc.TargetPath -like '*.exe') {
                $results += [PSCustomObject]@{n=$_.BaseName; p=$sc.TargetPath}
            }
        } catch {}
    }
}`
	}
	script += `
$results | ConvertTo-Json -Compress`

	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil
	}

	var entries []AppEntry
	if out[0] == '[' {
		var items []struct {
			N string `json:"n"`
			P string `json:"p"`
		}
		if json.Unmarshal(out, &items) == nil {
			for _, it := range items {
				if it.N != "" && it.P != "" {
					entries = append(entries, AppEntry{Name: it.N, Path: it.P})
				}
			}
		}
	} else {
		var item struct {
			N string `json:"n"`
			P string `json:"p"`
		}
		if json.Unmarshal(out, &item) == nil && item.N != "" && item.P != "" {
			entries = append(entries, AppEntry{Name: item.N, Path: item.P})
		}
	}
	return entries
}

// ============================================================
// Search engine (unchanged)
// ============================================================

type searchIndex struct {
	entries  []AppEntry
	pinyins  []string
	initials []string
}

func newSearchIndex(entries []AppEntry) *searchIndex {
	idx := &searchIndex{
		entries:  entries,
		pinyins:  make([]string, len(entries)),
		initials: make([]string, len(entries)),
	}
	for i, e := range entries {
		idx.pinyins[i] = strings.Join(pinyin.LazyConvert(e.Name, nil), "")
		idx.initials[i] = getInitials(e.Name)
	}
	return idx
}

// Search performs fuzzy + pinyin search on indexed apps.
func (li *LauncherIndex) Search(query string) []AppEntry {
	if query == "" {
		return li.entries[:min(30, len(li.entries))]
	}

	idx := newSearchIndex(li.entries)
	q := strings.ToLower(strings.TrimSpace(query))

	type scored struct {
		entry AppEntry
		score int
	}
	var results []scored

	for i, entry := range idx.entries {
		nameLower := strings.ToLower(entry.Name)
		pyFull := strings.ToLower(idx.pinyins[i])
		pyInit := strings.ToLower(idx.initials[i])
		pathLower := strings.ToLower(entry.Path)

		score := 0
		switch {
		case strings.EqualFold(entry.Name, q):
			score = 1000
		case strings.HasPrefix(nameLower, q):
			score = 500
		case strings.HasPrefix(pyFull, q):
			score = 400
		case strings.HasPrefix(pyInit, q):
			score = 350
		case strings.Contains(nameLower, q):
			score = 100
		case strings.Contains(pyFull, q):
			score = 80
		case strings.Contains(pyInit, q):
			score = 70
		case strings.Contains(pathLower, q):
			score = 30
		case fuzzyMatch(q, nameLower):
			score = 20
		case fuzzyMatch(q, pyInit):
			score = 15
		case fuzzyMatch(q, pyFull):
			score = 10
		}

		if score > 0 {
			results = append(results, scored{entry, score})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].score != results[j].score {
			return results[i].score > results[j].score
		}
		return results[i].entry.Name < results[j].entry.Name
	})

	out := make([]AppEntry, 0, min(8, len(results)))
	for i := 0; i < len(results) && i < 8; i++ {
		out = append(out, results[i].entry)
	}
	return out
}

// getInitials extracts the first letter of each word/character.
func getInitials(s string) string {
	var b strings.Builder
	inWord := false
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			if !inWord {
				b.WriteRune(r)
				inWord = true
			}
		} else if r >= 'A' && r <= 'Z' {
			if !inWord {
				b.WriteRune(r + 32)
				inWord = true
			}
		} else if unicode.Is(unicode.Han, r) {
			inWord = false
			py := pinyin.LazyConvert(string(r), &pinyin.Args{Style: pinyin.Initials})
			if len(py) > 0 && len(py[0]) > 0 {
				b.WriteByte(py[0][0])
			}
		} else {
			inWord = false
		}
	}
	return b.String()
}

// fuzzyMatch checks if pattern is a subsequence of text.
func fuzzyMatch(pattern, text string) bool {
	if len(pattern) == 0 {
		return true
	}
	if strings.HasPrefix(text, pattern) {
		return true
	}
	pi := 0
	for ti := 0; ti < len(text) && pi < len(pattern); ti++ {
		if text[ti] == pattern[pi] {
			pi++
		}
	}
	return pi == len(pattern)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
