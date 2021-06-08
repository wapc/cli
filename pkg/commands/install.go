package commands

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v33/github"
)

type InstallCmd struct {
	Location string `arg:"" help:"The NPM module or Github repository of the module to install."`
	Release  string `arg:"" help:"The release tag to install." optional:""`

	netClient http.Client
}

type releaseInfo struct {
	Org        string
	Module     string
	Tag        string
	Directory  string
	ZipURL     string
	TarballURL string
}

func (c *InstallCmd) Run(ctx *Context) error {
	homeDir, err := getHomeDirectory()
	if err != nil {
		return err
	}

	return c.doRun(ctx, homeDir)
}

func (c *InstallCmd) doRun(ctx *Context, homeDir string) error {
	if strings.Contains(c.Location, "..") {
		return fmt.Errorf("invalid location %s", c.Location)
	}

	c.createHTTPClient()

	release, err := c.getReleaseInfo(c.Location, c.Release)
	if err != nil {
		return err
	}

	fmt.Printf("Installing %s/%s %s...\n", release.Org, release.Module, release.Tag)

	if release.Directory != "" {
		moduleSubDir := release.Module
		if release.Org != "" {
			moduleSubDir = filepath.Join(release.Org, release.Module)
		}

		return c.installDir(
			release.Directory,
			homeDir,
			moduleSubDir,
		)
	}

	f, err := os.CreateTemp("", "install-*")
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(f.Name())
	}()

	var downloadURL string
	var fileType string
	if release.TarballURL != "" {
		downloadURL = release.TarballURL
		fileType = "tar.gz"
	} else if release.ZipURL != "" {
		downloadURL = release.ZipURL
		fileType = "zip"
	} else {
		return fmt.Errorf("release %s/%s %s does not contain a download URL",
			release.Org, release.Module, release.Tag)
	}

	resp, err := c.netClient.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	io.Copy(f, resp.Body)
	f.Close()

	downloadDir := filepath.Join(homeDir, "dl")
	os.RemoveAll(downloadDir)
	if err = os.MkdirAll(downloadDir, 0755); err != nil {
		return err
	}
	defer os.RemoveAll(downloadDir)

	fmt.Printf("Extracting %s...\n", filepath.Base(downloadURL))
	switch fileType {
	case "tar.gz":
		if err = c.extractTarball(f.Name(), downloadDir); err != nil {
			return err
		}
	case "zip":
		if err = c.extractZip(f.Name(), downloadDir); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown download type %s", fileType)
	}

	dirEntries, err := os.ReadDir(downloadDir)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if entry.IsDir() {
			contentsDir := filepath.Join(downloadDir, entry.Name())
			if err = readPackage(contentsDir, release); err != nil {
				return err
			}
			moduleSubDir := release.Module
			if release.Org != "" {
				moduleSubDir = filepath.Join(release.Org, release.Module)
			}

			if err = c.installDir(
				contentsDir,
				homeDir,
				moduleSubDir,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *InstallCmd) getReleaseInfo(location, releaseTag string) (*releaseInfo, error) {
	if strings.HasPrefix(location, "file:") {
		return c.getReleaseInfoFromDirectory(location[5:], releaseTag)
	}
	if strings.HasPrefix(location, "github.com/") {
		return c.getReleaseInfoFromGithub(location[11:], releaseTag)
	}

	return c.getReleaseInfoFromNPM(location, releaseTag)
}

func (c *InstallCmd) getReleaseInfoFromDirectory(location, releaseTag string) (*releaseInfo, error) {
	dir := filepath.Clean(location)
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}
	release := releaseInfo{
		Directory: dir,
	}
	if err = readPackage(dir, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

func (c *InstallCmd) getReleaseInfoFromNPM(location, releaseTag string) (*releaseInfo, error) {
	type dist struct {
		Tarball string `json:"tarball"`
	}
	type version struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Dist    dist   `json:"dist"`
	}

	if releaseTag == "" {
		releaseTag = "latest"
	}

	npmURL := fmt.Sprintf("https://registry.npmjs.org/%s/%s/", location, releaseTag)
	resp, err := c.netClient.Get(npmURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var v version
	if err = json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, err
	}

	var org string
	module := v.Name
	if strings.Contains(module, "..") {
		return nil, fmt.Errorf("invalid module name %s", module)
	}

	parts := strings.Split(v.Name, "/")
	if len(parts) == 2 {
		org = parts[0]
		module = parts[1]
	}

	return &releaseInfo{
		Org:        org,
		Module:     module,
		Tag:        v.Version,
		TarballURL: v.Dist.Tarball,
	}, nil
}

func (c *InstallCmd) getReleaseInfoFromGithub(location, releaseTag string) (*releaseInfo, error) {
	repoParts := strings.Split(location, "/")
	if len(repoParts) != 2 {
		return nil, fmt.Errorf("invalid repo syntax: %q", location)
	}

	org := repoParts[0]
	repo := repoParts[1]

	ct := context.Background()
	client := github.NewClient(nil)
	var release *github.RepositoryRelease

	if releaseTag == "" || releaseTag == "latest" {
		releases, _, err := client.Repositories.ListReleases(ct, org, repo, &github.ListOptions{
			PerPage: 1,
		})
		if err != nil {
			return nil, err
		}
		if len(releases) == 0 {
			return nil, fmt.Errorf("there are no releases for %s/%s", org, repo)
		}

		release = releases[0]
	} else {
		var err error
		release, _, err = client.Repositories.GetReleaseByTag(ct, org, repo, c.Release)
		if err != nil {
			if ghe, ok := err.(*github.ErrorResponse); ok && ghe.Response.StatusCode == 404 {
				branch, _, err := client.Repositories.GetBranch(ct, org, repo, c.Release)
				if err != nil {
					return nil, err
				}

				// Return download URL for a branch
				return &releaseInfo{
					Org:    org,
					Module: repo,
					Tag:    c.Release,
					ZipURL: fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip", org, repo, *branch.Name),
				}, nil
			}
			return nil, err
		}
	}

	if release.TagName == nil {
		return nil, fmt.Errorf("release tag is missing for %s/%s", org, repo)
	}

	info := releaseInfo{
		Org:    org,
		Module: repo,
		Tag:    *release.TagName,
	}

	if release.ZipballURL != nil {
		info.ZipURL = *release.ZipballURL
	}
	if release.ZipballURL != nil {
		info.TarballURL = *release.TarballURL
	}

	return &info, nil
}

var extensionDirectories = map[string]struct{}{
	"src":         {},
	"templates":   {},
	"definitions": {},
}

func (c *InstallCmd) installDir(src string, dest string, modulePart string) error {
	dirEntries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}

		base := filepath.Base(entry.Name())
		if _, ok := extensionDirectories[base]; ok {
			fmt.Printf("Copying into ~/.wapc/%s...\n", base)
			destDir := filepath.Join(dest, base, modulePart)
			if err = os.RemoveAll(destDir); err != nil {
				return err
			}
			if err = os.MkdirAll(destDir, 0755); err != nil {
				return err
			}
			if err = c.copyRecursive(
				filepath.Join(src, entry.Name()),
				destDir,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *InstallCmd) extractTarball(src string, dest string) error {
	r, err := os.OpenFile(src, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer r.Close()

	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := filepath.Join(dest, header.Name)

		// the following switch could also be done using fi.Mode(), not sure if there
		// a benefit of using one vs. the other.
		// fi := header.FileInfo()

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			dir := filepath.Dir(target)
			if _, err := os.Stat(dir); err != nil {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return err
				}
			}

			f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}

func (c *InstallCmd) extractZip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		// Check for ZipSlip. More Info: http://bit.ly/2MsjAWE
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("%s: illegal file path", fpath)
		}

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, 0755)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			return err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

func (c *InstallCmd) copyRecursive(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, ferr error) error {
		var relPath string = strings.Replace(path, source, "", 1)
		if relPath == "" {
			return nil
		}

		sourcePath := filepath.Join(source, relPath)
		stat, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			return os.Mkdir(filepath.Join(destination, relPath), stat.Mode())
		} else {
			data, err := os.ReadFile(sourcePath)
			if err != nil {
				return err
			}

			return os.WriteFile(filepath.Join(destination, relPath), data, stat.Mode())
		}
	})
}

func (c *InstallCmd) createHTTPClient() {
	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	c.netClient = http.Client{
		Timeout:   time.Second * 10,
		Transport: netTransport,
	}
}

func readPackage(dir string, release *releaseInfo) error {
	packageJSONPath := filepath.Join(dir, "package.json")
	packageJSONBytes, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return err
	}

	type packageJSON struct {
		Name string `json:"name"`
	}

	var contents packageJSON
	if err = json.Unmarshal(packageJSONBytes, &contents); err != nil {
		return err
	}

	if contents.Name == "" {
		return nil
	}

	parts := strings.Split(contents.Name, "/")
	if len(parts) > 2 {
		return nil
	}

	release.Org = parts[0]
	if len(parts) == 2 {
		release.Module = parts[1]
	} else {
		release.Module = ""
	}

	return nil
}
