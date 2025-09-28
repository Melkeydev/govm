package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type GoVersion struct {
	Version   string
	Filename  string
	URL       string
	Installed bool
	Active    bool
	Path      string
	Stable    bool
}
type SwitchCompletedMsg struct {
	Version    string
	ShimInPath bool
}
type DownloadCompleteMsg struct {
	Version string
	Path    string
}

func SetupShimDirectory() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}
	govmDir := filepath.Join(homeDir, ".govm")
	if err := os.MkdirAll(govmDir, 0755); err != nil {
		return fmt.Errorf("failed to create govm directory: %v", err)
	}
	shimDir := filepath.Join(govmDir, "shim")
	if err := os.MkdirAll(shimDir, 0755); err != nil {
		return fmt.Errorf("failed to create shim directory: %v", err)
	}
	return nil
}

// Check if user has shim
func IsShimInPath() bool {
	homeDir, _ := os.UserHomeDir()
	shimDir := filepath.Join(homeDir, ".govm", "shim")
	currentPath := os.Getenv("PATH")
	pathSeparator := string(os.PathListSeparator)
	pathEntries := strings.Split(currentPath, pathSeparator)
	for _, entry := range pathEntries {
		if entry == shimDir {
			return true
		}
	}
	return false
}
func GetShimPathInstructions() string {
	if runtime.GOOS == "windows" {
		return "Add to PATH: %USERPROFILE%\\.govm\\shim"
	} else {
		return "Add to your shell config: export PATH=\"$HOME/.govm/shim:$PATH\""
	}
}
func FetchGoVersions() tea.Msg {
	// I randomly put 10 second here
	client := &http.Client{
		Timeout: 10 * 1000000000,
	}
	resp, err := client.Get("https://go.dev/dl/?mode=json&include=all")
	if err != nil {
		return ErrMsg(fmt.Errorf("failed to connect to go.dev: %v", err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ErrMsg(err)
	}
	var releases []struct {
		Version string `json:"version"`
		Stable  bool   `json:"stable"`
		Files   []struct {
			Filename string `json:"filename"`
			OS       string `json:"os"`
			Arch     string `json:"arch"`
			Size     int    `json:"size"`
		} `json:"files"`
	}
	err = json.Unmarshal(body, &releases)
	if err != nil {
		return ErrMsg(fmt.Errorf("failed to parse API response: %v", err))
	}
	currentOS := runtime.GOOS
	arch := runtime.GOARCH
	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ErrMsg(err)
	}
	goVersionsDir := filepath.Join(homeDir, ".govm", "versions")
	err = os.MkdirAll(goVersionsDir, 0755)
	if err != nil {
		return ErrMsg(err)
	}
	activeVersion := ""
	activeVersionFile := filepath.Join(homeDir, ".govm", "active_version")
	if versionBytes, err := os.ReadFile(activeVersionFile); err == nil {
		activeVersion = string(versionBytes)
	} else {
		activeVersion = GetCurrentGoVersion()
	}
	installedVersions := map[string]string{}
	entries, _ := os.ReadDir(goVersionsDir)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "go") {
			versionPath := filepath.Join(goVersionsDir, entry.Name())
			version := strings.TrimPrefix(entry.Name(), "go")
			goBin := filepath.Join(versionPath, "bin", goBinary)
			if _, err := os.Stat(goBin); err == nil {
				installedVersions[version] = versionPath
			}
		}
	}
	var versions []GoVersion
	for _, release := range releases {
		version := strings.TrimPrefix(release.Version, "go")
		for _, file := range release.Files {
			if file.OS == currentOS && file.Arch == arch {
				v := GoVersion{
					Version:   version,
					Filename:  file.Filename,
					URL:       "https://go.dev/dl/" + file.Filename,
					Installed: false,
					Active:    false,
					Stable:    release.Stable,
				}
				if path, ok := installedVersions[version]; ok {
					v.Installed = true
					v.Path = path
				}
				if activeVersion == version {
					v.Active = true
				}
				versions = append(versions, v)
				break
			}
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		iParts := strings.Split(versions[i].Version, ".")
		jParts := strings.Split(versions[j].Version, ".")
		if len(iParts) > 0 && len(jParts) > 0 {
			iMajor, _ := strconv.Atoi(iParts[0])
			jMajor, _ := strconv.Atoi(jParts[0])
			if iMajor != jMajor {
				return iMajor > jMajor
			}
		}
		// Compare minor versions
		if len(iParts) > 1 && len(jParts) > 1 {
			iMinor, _ := strconv.Atoi(iParts[1])
			jMinor, _ := strconv.Atoi(jParts[1])
			if iMinor != jMinor {
				return iMinor > jMinor
			}
		}
		if len(iParts) > 2 && len(jParts) > 2 {
			iPatch, _ := strconv.Atoi(iParts[2])
			jPatch, _ := strconv.Atoi(jParts[2])
			return iPatch > jPatch
		}
		return versions[i].Version > versions[j].Version
	})
	return VersionsMsg(versions)
}
func GetCurrentGoVersion() string {
	cmd := exec.Command("go", "version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	parts := strings.Split(string(output), " ")
	if len(parts) >= 3 {
		return strings.TrimPrefix(parts[2], "go")
	}
	return ""
}
func DownloadAndInstall(version GoVersion) tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ErrMsg(err)
		}
		goVersionsDir := filepath.Join(homeDir, ".govm", "versions")
		downloadDir := filepath.Join(homeDir, ".govm", "downloads")
		for _, dir := range []string{goVersionsDir, downloadDir} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return ErrMsg(err)
			}
		}
		versionDir := filepath.Join(goVersionsDir, fmt.Sprintf("go%s", version.Version))
		if _, err := os.Stat(versionDir); err == nil {
			if err := os.RemoveAll(versionDir); err != nil {
				return ErrMsg(fmt.Errorf("failed to remove existing installation: %v", err))
			}
		}
		downloadPath := filepath.Join(downloadDir, version.Filename)
		if _, err := os.Stat(downloadPath); err == nil {
			if err := os.Remove(downloadPath); err != nil {
				return ErrMsg(fmt.Errorf("failed to remove existing download: %v", err))
			}
		}
		resp, err := http.Get(version.URL)
		if err != nil {
			return ErrMsg(err)
		}
		defer resp.Body.Close()
		out, err := os.Create(downloadPath)
		if err != nil {
			return ErrMsg(err)
		}
		defer out.Close()
		written, err := io.Copy(out, resp.Body)
		if err != nil {
			return ErrMsg(err)
		}
		if written == 0 {
			return ErrMsg(fmt.Errorf("downloaded empty file"))
		}
		out.Close()
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			if strings.HasSuffix(version.Filename, ".zip") {
				cmd = exec.Command("powershell", "-Command",
					fmt.Sprintf("Expand-Archive -Path \"%s\" -DestinationPath \"%s\" -Force",
						downloadPath, goVersionsDir))
			} else {
				return ErrMsg(fmt.Errorf("unsupported archive format for Windows: %s", version.Filename))
			}
		} else {
			if strings.HasSuffix(version.Filename, ".tar.gz") {
				cmd = exec.Command("tar", "-xzf", downloadPath, "-C", goVersionsDir)
			} else {
				return ErrMsg(fmt.Errorf("unsupported archive format for Unix: %s", version.Filename))
			}
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return ErrMsg(fmt.Errorf("extraction error: %v\nOutput: %s", err, string(output)))
		}

		if runtime.GOOS != "windows" {
			goBin := filepath.Join(versionDir, "bin", goBinary)
			if _, err := os.Stat(goBin); err == nil {
				os.Chmod(goBin, 0755)
			}
		}
		goBin := filepath.Join(versionDir, "bin", goBinary)
		if _, err := os.Stat(goBin); os.IsNotExist(err) {
			entries, _ := os.ReadDir(goVersionsDir)
			for _, entry := range entries {
				if entry.IsDir() && strings.HasPrefix(entry.Name(), "go") {
					testPath := filepath.Join(goVersionsDir, entry.Name(), "bin", goBinary)
					if _, err := os.Stat(testPath); err == nil {
						sourcePath := filepath.Join(goVersionsDir, entry.Name())
						if sourcePath != versionDir {
							if err := os.Rename(sourcePath, versionDir); err != nil {
								return ErrMsg(fmt.Errorf("failed to rename directory: %v", err))
							}
						}
						break
					}
				}
			}
		}
		if _, err := os.Stat(goBin); os.IsNotExist(err) {
			return ErrMsg(fmt.Errorf("installation failed: Go binary not found at %s", goBin))
		}
		verifyCmd := exec.Command(goBin, "version")
		verifyOutput, err := verifyCmd.CombinedOutput()
		if err != nil {
			return ErrMsg(fmt.Errorf("Go binary verification failed: %v\nOutput: %s", err, string(verifyOutput)))
		}
		// Remove the existing downloads since they should be installed
		if err := os.Remove(downloadPath); err != nil {
			// Just log the error but don't fail the installation
			fmt.Printf("Warning: failed to clean up download file: %v\n", err)
		}
		return DownloadCompleteMsg{Version: version.Version, Path: versionDir}
	}
}
func SwitchVersion(version GoVersion) tea.Cmd {
	return func() tea.Msg {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ErrMsg(err)
		}
		if err := SetupShimDirectory(); err != nil {
			return ErrMsg(err)
		}
		shimDir := filepath.Join(homeDir, ".govm", "shim")
		versionBinDir := filepath.Join(version.Path, "bin")
		if _, err := os.Stat(versionBinDir); os.IsNotExist(err) {
			return ErrMsg(fmt.Errorf("go version directory not found: %s", versionBinDir))
		}
		entries, err := os.ReadDir(versionBinDir)
		if err != nil {
			return ErrMsg(fmt.Errorf("failed to read bin directory: %v", err))
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				binName := strings.Trim(entry.Name(), ".exe")
				targetBin := filepath.Join(versionBinDir, binName)
				shimPath := filepath.Join(shimDir, binName)
				os.Remove(shimPath)
				if runtime.GOOS == "windows" {
					shimContent := fmt.Sprintf(`@echo off
"%s" %%*
`, targetBin)
					if err := os.WriteFile(shimPath+".bat", []byte(shimContent), 0755); err != nil {
						return ErrMsg(fmt.Errorf("failed to create shim for %s: %v", binName, err))
					}
				} else {
					shimContent := fmt.Sprintf(`#!/usr/bin/env bash
"%s" "$@"
`, targetBin)
					if err := os.WriteFile(shimPath, []byte(shimContent), 0755); err != nil {
						return ErrMsg(fmt.Errorf("failed to create shim for %s: %v", binName, err))
					}
					if err := os.Chmod(shimPath, 0755); err != nil {
						return ErrMsg(fmt.Errorf("failed to make shim executable: %v", err))
					}
				}
			}
		}
		versionFile := filepath.Join(homeDir, ".govm", "active_version")
		if err := os.WriteFile(versionFile, []byte(version.Version), 0644); err != nil {
			return ErrMsg(fmt.Errorf("failed to update active version file: %v", err))
		}
		shimInPath := IsShimInPath()
		return SwitchCompletedMsg{
			Version:    version.Version,
			ShimInPath: shimInPath,
		}
	}
}

func DeleteVersion(version GoVersion) tea.Cmd {
	return func() tea.Msg {
		if !version.Installed {
			return ErrMsg(fmt.Errorf("version %s is not installed", version.Version))
		}

		if version.Active {
			return ErrMsg(fmt.Errorf("cannot delete active version - switch to another version first"))
		}

		if err := os.RemoveAll(version.Path); err != nil {
			return ErrMsg(fmt.Errorf("failed to delete version %s: %v", version.Version, err))
		}

		return DeleteCompleteMsg{Version: version.Version}
	}
}
