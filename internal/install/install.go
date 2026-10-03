// Package install manages external persona repositories (clone, update, uninstall).
package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/luuuc/council/internal/expert"
)

// Install clones a git repository to the installed directory.
func Install(url string) (string, error) {
	installedDir, err := InstalledPath()
	if err != nil {
		return "", err
	}

	// Ensure installed directory exists
	if err := os.MkdirAll(installedDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create installed directory: %w", err)
	}

	// Extract repo name from URL
	name := repoNameFromURL(url)
	if name == "" {
		return "", fmt.Errorf("could not extract repository name from URL")
	}

	destPath := filepath.Join(installedDir, name)

	// Check if already installed
	if _, err := os.Stat(destPath); err == nil {
		return "", fmt.Errorf("repository '%s' is already installed\n\nUpdate with: council personas update %s", name, name)
	}

	// Clone repository
	cmd := exec.Command("git", "clone", url, destPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %w", err)
	}

	return name, nil
}

// repoNameFromURL extracts a safe directory name from a git URL.
// https://github.com/user/my-council.git -> user-my-council
// git@github.com:user/my-council.git -> user-my-council
func repoNameFromURL(url string) string {
	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Extract user/repo from various URL formats
	var user, repo string

	// HTTPS: https://github.com/user/repo
	httpsRegex := regexp.MustCompile(`https?://[^/]+/([^/]+)/([^/]+)/?$`)
	if matches := httpsRegex.FindStringSubmatch(url); len(matches) == 3 {
		user, repo = matches[1], matches[2]
	}

	// SSH: git@github.com:user/repo
	sshRegex := regexp.MustCompile(`git@[^:]+:([^/]+)/([^/]+)/?$`)
	if matches := sshRegex.FindStringSubmatch(url); len(matches) == 3 {
		user, repo = matches[1], matches[2]
	}

	if user == "" || repo == "" {
		return ""
	}

	return user + "-" + repo
}

// Uninstall removes an installed repository.
func Uninstall(name string) error {
	installedDir, err := InstalledPath()
	if err != nil {
		return err
	}

	destPath := filepath.Join(installedDir, name)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("repository '%s' is not installed", name)
	}

	return os.RemoveAll(destPath)
}

// Update pulls the latest changes for an installed repository.
func Update(name string) error {
	installedDir, err := InstalledPath()
	if err != nil {
		return err
	}

	destPath := filepath.Join(installedDir, name)

	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		return fmt.Errorf("repository '%s' is not installed", name)
	}

	repo := NewGitRepo(destPath)
	return repo.Pull()
}

// UpdateAll updates all installed repositories.
func UpdateAll() ([]string, error) {
	installedDir, err := InstalledPath()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(installedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var updated []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if err := Update(name); err != nil {
			fmt.Printf("Warning: failed to update %s: %v\n", name, err)
			continue
		}
		updated = append(updated, name)
	}

	return updated, nil
}

// ListInstalled returns the names of all installed repositories.
func ListInstalled() ([]string, error) {
	installedDir, err := InstalledPath()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(installedDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}

	return names, nil
}

// ListInstalledExperts returns all experts from installed repositories.
func ListInstalledExperts() ([]*expert.Expert, error) {
	installedDir, err := InstalledPath()
	if err != nil {
		return nil, err
	}

	installed, err := ListInstalled()
	if err != nil {
		return nil, err
	}

	var experts []*expert.Expert
	for _, name := range installed {
		repoPath := filepath.Join(installedDir, name)
		repoExperts, err := ListExpertsInDir(repoPath, "installed:"+name)
		if err != nil {
			continue
		}
		experts = append(experts, repoExperts...)
	}

	return experts, nil
}
