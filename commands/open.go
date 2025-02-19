package commands

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/wowu/pro/config"
	"github.com/wowu/pro/providers/github"
	"github.com/wowu/pro/providers/gitlab"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	giturls "github.com/whilp/git-urls"
)

func Open(repoPath string, print bool) {
	repository, err := findRepo(repoPath)
	if err != nil {
		color.Red("Unable to find git repository in given directory or any of parent directories.")
		fmt.Println("Please make sure you are in the project directory.")
		os.Exit(1)
	}

	// check if there is a remote named origin
	origin, err := repository.Remote("origin")
	if err != nil {
		color.Red("No remote named \"origin\" found.")
		fmt.Println("Please make sure you have a remote named \"origin\".")
		os.Exit(1)
	}

	// get current head
	head, err := repository.Head()
	handleError(err, "Unable to get repository head")

	if !head.Name().IsBranch() {
		color.Red("No active branch found.")
		fmt.Println("Switch to a branch and try again.")
		os.Exit(0)
	}

	// get current branch name
	branch := head.Name().Short()
	fmt.Printf("Current branch: %s\n", color.GreenString(branch))

	originURL := origin.Config().URLs[0]

	gitURL, err := giturls.Parse(originURL)
	handleError(err, "Unable to parse origin URL")

	if branch == "master" || branch == "main" || branch == "trunk" || branch == "develop" {
		fmt.Println("Looks like you are on the main branch. Opening home page.")

		homeUrl := fmt.Sprintf("https://%s/%s", gitURL.Host, strings.TrimPrefix(gitURL.Path, "/"))
		homeUrl = strings.TrimSuffix(homeUrl, ".git")

		if print {
			color.Blue(homeUrl)
		} else {
			color.Blue(homeUrl)
			openBrowser(homeUrl)
		}

		os.Exit(0)
	}

	projectPath := strings.TrimPrefix(gitURL.Path, "/")
	projectPath = strings.TrimSuffix(projectPath, ".git")

	switch gitURL.Host {
	case "gitlab.com":
		openGitLab(branch, projectPath, print)
	case "github.com":
		openGitHub(branch, projectPath, print)
	default:
		fmt.Println("Unknown remote type")
		os.Exit(1)
	}
}

// Find git repository in given directory or parent directories
func findRepo(path string) (*git.Repository, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	repository, err := git.PlainOpen(absolutePath)

	if err == nil {
		return repository, nil
	}

	if errors.Is(err, git.ErrRepositoryNotExists) {
		// Base case - we've reached the root of the filesystem
		if absolutePath == "/" {
			return nil, errors.New("no git repository found")
		}

		// Recurse to parent directory
		return findRepo(filepath.Dir(absolutePath))
	}

	return nil, err
}

func openGitLab(branch string, projectPath string, print bool) {
	gitlabToken := config.Get().GitLabToken

	if gitlabToken == "" {
		color.Red("GitLab token is not set. Run `pro auth gitlab` to set it.")
		os.Exit(1)
	}

	mergeRequest, err := gitlab.FindMergeRequest(projectPath, gitlabToken, branch)
	if err != nil {
		if errors.Is(err, gitlab.ErrNotFound) {
			fmt.Println("No open merge request found for current branch")
			fmt.Println("Create pull request at", color.BlueString("https://gitlab.com/%s/merge_requests/new?merge_request%%5Bsource_branch%%5D=%s", projectPath, branch))
			os.Exit(0)
		} else if errors.Is(err, gitlab.ErrUnauthorized) || errors.Is(err, gitlab.ErrTokenExpired) {
			color.Red("Unable to get merge requests: %s", err.Error())
			fmt.Println("Connect GitLab again with `pro auth gitlab`.")
			os.Exit(1)
		} else {
			color.Red("Unable to get merge requests: %s", err.Error())
			os.Exit(1)
		}
	}

	url := mergeRequest.WebUrl

	if print {
		color.Blue(url)
	} else {
		fmt.Println("Opening " + color.BlueString(url))
		openBrowser(url)
	}
}

func openGitHub(branch string, projectPath string, print bool) {
	githubToken := config.Get().GitHubToken

	if githubToken == "" {
		color.Red("GitHub token is not set. Run `pro auth github` to set it.")
		os.Exit(1)
	}

	pullRequest, err := github.FindPullRequest(projectPath, githubToken, branch)
	if err != nil {
		if errors.Is(err, github.ErrNotFound) {
			fmt.Println("No open pull request found for current branch")
			fmt.Println("Create pull request at", color.BlueString("https://github.com/%s/pull/new/%s", projectPath, branch))
			os.Exit(0)
		} else if errors.Is(err, github.ErrUnauthorized) {
			color.Red("Unable to get pull requests: %s", err.Error())
			fmt.Println("Token may be expired or deleted. Run `pro auth github` to connect GitHub again.")
			os.Exit(1)
		} else {
			color.Red("Unable to get pull requests: %s", err.Error())
			os.Exit(1)
		}
	}

	url := pullRequest.HtmlURL

	if print {
		color.Blue(url)
	} else {
		fmt.Println("Opening " + color.BlueString(url))
		openBrowser(url)
	}
}

func openBrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}

	if err != nil {
		fmt.Printf("Unable to open browser: %s\n", err)
		os.Exit(1)
	}
}
