package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRepoNameFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "https github url",
			url:      "https://github.com/ben/my-council.git",
			expected: "ben-my-council",
		},
		{
			name:     "https github url without .git",
			url:      "https://github.com/ben/my-council",
			expected: "ben-my-council",
		},
		{
			name:     "ssh github url",
			url:      "git@github.com:ben/my-council.git",
			expected: "ben-my-council",
		},
		{
			name:     "ssh github url without .git",
			url:      "git@github.com:ben/my-council",
			expected: "ben-my-council",
		},
		{
			name:     "https gitlab url",
			url:      "https://gitlab.com/user/repo.git",
			expected: "user-repo",
		},
		{
			name:     "trailing slash",
			url:      "https://github.com/ben/my-council/",
			expected: "ben-my-council",
		},
		{
			name:     "invalid url - no user",
			url:      "https://github.com/my-council.git",
			expected: "",
		},
		{
			name:     "invalid url - not a git url",
			url:      "not-a-url",
			expected: "",
		},
		{
			name:     "empty string",
			url:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := repoNameFromURL(tt.url)
			if result != tt.expected {
				t.Errorf("repoNameFromURL(%q) = %q, want %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestInstalledPath(t *testing.T) {
	path, err := InstalledPath()
	if err != nil {
		t.Fatalf("InstalledPath() error = %v", err)
	}
	if path == "" {
		t.Error("InstalledPath() returned empty string")
	}
	// Should end with .council/installed
	if !filepath.IsAbs(path) {
		t.Errorf("InstalledPath() = %q, want absolute path", path)
	}
}

func TestListInstalled_EmptyWhenNoDir(t *testing.T) {
	// Get the actual installed path
	installedPath, err := InstalledPath()
	if err != nil {
		t.Fatalf("InstalledPath() error = %v", err)
	}

	// Skip if the installed directory already exists with content
	if entries, err := os.ReadDir(installedPath); err == nil && len(entries) > 0 {
		t.Skip("installed directory already has content, skipping empty test")
	}

	installed, err := ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}
	// If the directory doesn't exist or is empty, should return empty or nil slice
	if len(installed) != 0 {
		t.Errorf("ListInstalled() returned %d items, want 0", len(installed))
	}
}

func TestListInstalled_ReturnsDirectories(t *testing.T) {
	// Get the actual installed path
	installedPath, err := InstalledPath()
	if err != nil {
		t.Fatalf("InstalledPath() error = %v", err)
	}

	// Create test directories in the actual installed path
	testRepo1 := filepath.Join(installedPath, "test-repo-1-"+t.Name())
	testRepo2 := filepath.Join(installedPath, "test-repo-2-"+t.Name())

	if err := os.MkdirAll(testRepo1, 0755); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(testRepo1) }()

	if err := os.MkdirAll(testRepo2, 0755); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(testRepo2) }()

	installed, err := ListInstalled()
	if err != nil {
		t.Fatalf("ListInstalled() error = %v", err)
	}

	// Check that our test directories are in the list
	found1, found2 := false, false
	for _, name := range installed {
		if name == filepath.Base(testRepo1) {
			found1 = true
		}
		if name == filepath.Base(testRepo2) {
			found2 = true
		}
	}

	if !found1 || !found2 {
		t.Errorf("ListInstalled() did not return expected test directories")
	}
}

func TestGitRepo_IsRepo_FalseWhenNoGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewGitRepo(tmpDir)

	if repo.IsRepo() {
		t.Error("IsRepo() = true for non-git directory, want false")
	}
}

func TestGitRepo_IsRepo_TrueWhenGitDirExists(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatal(err)
	}

	repo := NewGitRepo(tmpDir)

	if !repo.IsRepo() {
		t.Error("IsRepo() = false for git directory, want true")
	}
}

func TestGitRepo_Pull_ErrorWhenNotRepo(t *testing.T) {
	tmpDir := t.TempDir()
	repo := NewGitRepo(tmpDir)

	err := repo.Pull()
	if err == nil {
		t.Error("Pull() on non-repo should return error")
	}
	if err.Error() != "not a git repository" {
		t.Errorf("Pull() error = %q, want 'not a git repository'", err.Error())
	}
}

func TestUninstall_ErrorWhenNotInstalled(t *testing.T) {
	err := Uninstall("nonexistent-repo-" + t.Name())
	if err == nil {
		t.Error("Uninstall() should return error for nonexistent repo")
	}
}

func TestUninstall_RemovesDirectory(t *testing.T) {
	// Get the actual installed path
	installedPath, err := InstalledPath()
	if err != nil {
		t.Fatalf("InstalledPath() error = %v", err)
	}

	// Create a test repo in the actual installed path
	testRepoName := "test-uninstall-" + t.Name()
	testRepoPath := filepath.Join(installedPath, testRepoName)

	if err := os.MkdirAll(testRepoPath, 0755); err != nil {
		t.Fatal(err)
	}

	// Verify it exists
	if _, err := os.Stat(testRepoPath); os.IsNotExist(err) {
		t.Fatal("test repo directory was not created")
	}

	err = Uninstall(testRepoName)
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	// Verify it was removed
	if _, err := os.Stat(testRepoPath); !os.IsNotExist(err) {
		t.Error("Uninstall() did not remove the directory")
	}
}

func TestUpdate_ErrorWhenNotInstalled(t *testing.T) {
	err := Update("nonexistent-repo-" + t.Name())
	if err == nil {
		t.Error("Update() should return error for nonexistent repo")
	}
}
