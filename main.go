package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/davecgh/go-spew/spew"
	"github.com/google/go-github/v28/github"
	"github.com/hashicorp/go-multierror"
	"github.com/jinzhu/copier"
)

var _ = spew.Dump

type Stability string

const (
	Stable Stability = "stable"
	RC     Stability = "rc"
	Beta   Stability = "beta"
	Alpha  Stability = "alpha"
)

type Dependency struct {
	Github  *GithubConfig `json:",omitempty"`
	Gitlab  *GitlabConfig `json:",omitempty"`
	Version string
	Options struct {
		Tags         []string `json:",omitempty"`
		MinStability Stability
	}
}

type GithubConfig struct {
	Owner string
	Repo  string
}
type GitlabConfig struct {
	Owner string
	Repo  string
}

type Config struct {
	Tags         []string `json:",omitempty"`
	Dependencies map[string]Dependency
}

func main() {
	ctx := context.Background()

	// client := github.NewClient(nil)

	// installs, _, err := client.Apps.ListInstallations(ctx, nil)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// for _, install := range installs {
	var id int64 = 3753444
	install := &github.Installation{ID: &id}
	if err := CheckInstall(ctx, install); err != nil {
		log.Fatal(err)
	}
	// }
}

func CheckDepenency(ctx context.Context, client *github.Client, repo *github.Repository, blobSHA string, conf Config, name string, c Dependency) error {
	r, _, err := client.Repositories.ListReleases(ctx, c.Github.Owner, c.Github.Repo, &github.ListOptions{PerPage: 1})
	if err != nil {
		return err
	}

	sha := "f61668e1c9894ae549d3e5e8d15d501ea532877b"

	for _, release := range r {
		if release.GetTagName() != c.Version {

			c.Version = release.GetTagName()

			conf.Dependencies[name] = c

			b, _ := json.MarshalIndent(conf, "", "  ")
			message := fmt.Sprintf("Update .git-dependencies.json with update for %s", name)
			branch := fmt.Sprintf("git-deps-update-%s-%s", name, c.Version)
			authorName := "Git Deps Updater"
			autherEmail := "deps@ttp.sh"

			refName := fmt.Sprintf("refs/heads/%s", branch)

			// log.Fatal("hi")

			ref := github.Reference{
				Ref: &refName,
				Object: &github.GitObject{
					SHA: &sha,
				},
			}

			_, _, err := client.Git.CreateRef(ctx, repo.Owner.GetLogin(), repo.GetName(), &ref)
			if err != nil {
				return err
			}

			l, _, err := client.Repositories.UpdateFile(ctx,
				repo.Owner.GetLogin(),
				repo.GetName(),
				".git-dependencies.json",
				&github.RepositoryContentFileOptions{
					Message: &message,
					Content: b,
					Branch:  &branch,
					SHA:     &blobSHA,
					Committer: &github.CommitAuthor{
						Name:  &authorName,
						Email: &autherEmail,
					},
				},
			)
			if err != nil {
				return err
			}

			_ = l

			pull := github.NewPullRequest{
				Title: str("Update Deps"),
				Head:  &branch,
				Base:  str("master"),
			}
			_, _, err = client.PullRequests.Create(ctx, repo.Owner.GetLogin(), repo.GetName(), &pull)
			if err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

func CheckRepo(ctx context.Context, client *github.Client, repo *github.Repository) error {

	c, _, _, err := client.Repositories.GetContents(
		ctx,
		repo.Owner.GetLogin(),
		repo.GetName(),
		".git-dependencies.json",
		&github.RepositoryContentGetOptions{
			Ref: "master",
		},
	)
	if err != nil {
		return err
	}

	blobSha := c.GetSHA()

	content, _ := c.GetContent()
	conf := Config{}
	if err := json.Unmarshal([]byte(content), &conf); err != nil {
		return err
	}

	var errors error
	for name, dep := range conf.Dependencies {
		log.Printf("Found dep %s", name)
		var a Config
		copier.Copy(&a, &conf)
		if err := CheckDepenency(ctx, client, repo, blobSha, a, name, dep); err != nil {
			errors = multierror.Append(errors, err)
		}
	}
	return errors
}

func CheckInstall(ctx context.Context, install *github.Installation) error {
	filename := "git-dependency-watcher.2019-10-22.private-key.pem"
	// Shared transport to reuse TCP connections.
	tr := http.DefaultTransport

	// Wrap the shared transport for use with the app ID 1 authenticating with installation ID 99.
	itr, err := ghinstallation.NewKeyFromFile(tr, 44432, int(*install.ID), filename)
	if err != nil {
		return err
	}

	httpClient := &http.Client{Transport: itr}
	// Use installation transport with github.com/google/go-github
	client := github.NewClient(httpClient)

	repos, _, err := client.Apps.ListRepos(ctx, nil)
	if err != nil {
		return err
	}

	var errors error
	for _, repo := range repos {
		log.Printf("Found repo %s", repo.GetName())
		if err := CheckRepo(ctx, client, repo); err != nil {
			errors = multierror.Append(errors, err)
		}
	}
	return errors
}

func str(v string) *string {
	return &v
}
