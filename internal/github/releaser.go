package github

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/Masterminds/semver/v3"
	"github.com/google/go-github/v42/github"
	"github.com/paulfarver/valet/internal/chart"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

type ReleaserConfig struct {
	Rules []RuleConfig `yaml:"rules"`
}

type RuleConfig struct {
	Branch   string `yaml:"branch"`
	Files    string `yaml:"files"`
	Strategy string `yaml:"strategy"`
}

type Rule struct {
	Branch   string
	Files    *regexp.Regexp
	Indent   int
	Strategy string // Has no effect yet. TODO: implement
}

const (
	StrategyPullRequest = "pull-request"
	StrategyDirect      = "direct"
)

var ErrFileMissing = errors.New("File missing in repository")

// Releaser is a configured client for updating files in a repository
type Releaser struct {
	Client       *github.Client
	Repository   *github.Repository
	Rules        []Rule
	log          logrus.FieldLogger
	chartService chart.Service
}

func (s *Service) NewReleaser(ctx context.Context, client *github.Client, repo *github.Repository, log logrus.FieldLogger, chartService chart.Service) (*Releaser, error) {
	l := log.WithField("repository", repo.GetFullName()).WithField("component", "releaser")

	file := s.config.ReleaseConfigPath
	ref := repo.GetDefaultBranch()
	l.Infof("Reading %s from %s", file, ref)
	r, o, err := client.Repositories.DownloadContents(ctx, repo.GetOwner().GetLogin(), repo.GetName(), file, &github.RepositoryContentGetOptions{
		Ref: ref,
	})
	if o.StatusCode == http.StatusNotFound {
		return nil, ErrFileMissing
	}
	if err != nil {
		if errors.Is(err, fmt.Errorf("No file named %s found in %s", path.Base(file), path.Dir(file))) {
			l.Warn(err)
			return nil, ErrFileMissing
		}
		return nil, errors.Wrap(err, "Failed to read file")
	}
	defer r.Close()

	var config ReleaserConfig
	if err := yaml.NewDecoder(r).Decode(&config); err != nil {
		return nil, errors.Wrap(err, "Failed to decode config")
	}
	// TODO: validate config

	rules, err := readRules(config.Rules)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read rules")
	}

	return &Releaser{
		Client:       client,
		Repository:   repo,
		Rules:        rules,
		log:          l,
		chartService: chartService,
	}, nil
}

func readRules(ruleConfigs []RuleConfig) ([]Rule, error) {
	var rules []Rule
	for _, r := range ruleConfigs {
		files, err := regexp.Compile(r.Files)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to compile files regexp %s", r.Files)
		}
		rules = append(rules, Rule{
			Branch:   r.Branch,
			Files:    files,
			Strategy: r.Strategy,
		})
	}
	return rules, nil
}

func (r *Releaser) ScanAndUpdate(ctx context.Context) error {
	for _, rule := range r.Rules {
		if err := r.ScanAndUpdateWithRule(ctx, rule); err != nil {
			r.log.WithError(err).Warn("Failed to scan and update with rule")
			continue
		}
	}
	return nil
}

func (r *Releaser) ScanAndUpdateWithRule(ctx context.Context, rule Rule) error {
	ref, _, err := r.Client.Git.GetRef(ctx, r.Repository.GetOwner().GetLogin(), r.Repository.GetName(), fmt.Sprintf("heads/%s", rule.Branch))
	if err != nil {
		return errors.Wrap(err, "Failed to get ref")
	}

	tree, _, err := r.Client.Git.GetTree(ctx, r.Repository.Owner.GetLogin(), r.Repository.GetName(), ref.Object.GetSHA(), true)
	if err != nil {
		return errors.Wrap(err, "Failed to get tree")
	}

	for _, entry := range tree.Entries {
		if entry.GetType() == "blob" {
			if rule.Files.MatchString(entry.GetPath()) {
				r.log.Infof("Found matching file %s %s", entry.GetPath(), entry.GetSHA())
				if err := r.UpdateFile(ctx, entry, ref); err != nil {
					r.log.WithError(err).Warn("Failed to update file")
				}
			}
		}
	}
	return nil
}

func (r *Releaser) UpdateFile(ctx context.Context, entry *github.TreeEntry, ref *github.Reference) error {
	r.log.Infof("Downloading file %s", entry.GetURL())
	b, _, err := r.Client.Git.GetBlobRaw(ctx, r.Repository.GetOwner().GetLogin(), r.Repository.GetName(), entry.GetSHA())
	if err != nil {
		return errors.Wrap(err, "Failed to get file")
	}

	reader := bytes.NewReader(b)
	decoder := yaml.NewDecoder(reader)
	documents := []*gabs.Container{}
	for {
		var m map[string]interface{}
		if err := decoder.Decode(&m); err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrap(err, "Failed to decode yaml")
		}
		documents = append(documents, gabs.Wrap(m))
	}

	buf := bytes.NewBuffer([]byte{})
	encoder := yaml.NewEncoder(buf)
	encoder.SetIndent(2)

	updateRequired := false

	for _, doc := range documents {
		updated, err := r.UpdateDocument(ctx, doc)
		if err != nil {
			r.log.WithError(err).Warn("Failed to update document")
			updated = doc
		} else {
			updateRequired = true
		}

		if err := encoder.Encode(updated.Data()); err != nil {
			return errors.Wrap(err, "Failed to encode yaml")
		}
	}

	if updateRequired {
		branchName := fmt.Sprintf("valet/%s/bump", entry.GetPath())
		if len(entry.GetPath()) > 50 {
			branchName = fmt.Sprintf("valet/%s/bump", entry.GetPath()[len(entry.GetPath())-50:])
		}

		_, _, err := r.Client.Git.CreateRef(ctx, r.Repository.GetOwner().GetLogin(), r.Repository.GetName(), &github.Reference{
			Ref: github.String(fmt.Sprintf("heads/%s", branchName)),
			Object: &github.GitObject{
				SHA: ref.Object.SHA,
			},
		})
		if err != nil {
			return errors.Wrap(err, "Failed to create ref")
		}
		_, _, err = r.Client.Repositories.UpdateFile(ctx, r.Repository.GetOwner().GetLogin(), r.Repository.GetName(), entry.GetPath(), &github.RepositoryContentFileOptions{
			Message: github.String(fmt.Sprintf("Bump chart in %s", entry.GetPath())),
			Content: buf.Bytes(),
			Branch:  &branchName,
			SHA:     entry.SHA,
		})
		if err != nil {
			return errors.Wrap(err, "Failed to update file")
		}
		_, _, err = r.Client.PullRequests.Create(ctx, r.Repository.GetOwner().GetLogin(), r.Repository.GetName(), &github.NewPullRequest{
			Title: github.String(fmt.Sprintf("Bump chart in %s", entry.GetPath())),
			Head:  github.String(branchName),
			Base:  ref.Ref,
		})
		if err != nil {
			return errors.Wrap(err, "Failed to create pull request")
		}
	}

	return nil
}

var ErrNotAutomated = errors.New("Not an automated release")

var (
	filterChart   = regexp.MustCompile(`^filter.valet.io/chart$`)
	filterRegex   = regexp.MustCompile(`^filter.valet.io/(.+)$`)
	registryRegex = regexp.MustCompile(`^registry.valet.io/(.+)$`)
	tagRegex      = regexp.MustCompile(`^tag.valet.io/(.+)$`)
)

type updateRule struct{}

func (r *Releaser) UpdateDocument(ctx context.Context, doc *gabs.Container) (*gabs.Container, error) {
	if !doc.Exists("metadata", "annotations", "valet.io/automated") {
		return nil, ErrNotAutomated
	}

	if doc.Search("metadata", "annotations", "valet.io/automated").Data() != "true" {
		return nil, ErrNotAutomated
	}

	con, _ := semver.NewConstraint(">=0.0.0")

	for key, value := range doc.Search("metadata", "annotations").ChildrenMap() {
		if filterChart.MatchString(key) {
			str, ok := value.Data().(string)
			if !ok {
				return nil, errors.New("Invalid filter type")
			}

			r.log.Infof("Found filter for chart %s", str)

			vals := strings.SplitN(str, ":", 2)
			if len(vals) != 2 {
				return nil, errors.Errorf("Invalid filter %s", str)
			}

			switch vals[0] {
			case "semver":
				var err error
				con, err = semver.NewConstraint(vals[1])
				if err != nil {
					return nil, errors.Wrap(err, "Failed to parse constraint")
				}
			default:
				return nil, errors.Errorf("Unknown filter type %s", vals[0])
			}
			continue
		}
	}

	name, ok := doc.Search("spec", "chart", "name").Data().(string)
	if !ok {
		return nil, errors.New("Failed to get chart name")
	}
	repo, ok := doc.Search("spec", "chart", "repository").Data().(string)
	if !ok {
		return nil, errors.New("Failed to get chart repository")
	}
	currVersion, ok := doc.Search("spec", "chart", "version").Data().(string)
	if !ok {
		return nil, errors.New("Failed to get chart version")
	}

	oldV, err := semver.NewVersion(currVersion)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to current version")
	}

	available, err := r.chartService.ListVersions(ctx, repo, name)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list available chart versions")
	}

	v := oldV
	for _, version := range available {
		v2, err := semver.NewVersion(version)
		if err != nil {
			r.log.WithError(err).Warnf("Failed to parse version %s", version)
			continue
		}
		if v2.GreaterThan(v) && con.Check(v2) {
			v = v2
		}
	}

	if v.Equal(oldV) {
		return nil, errors.New("No new version found")
	}

	doc.Set(v.String(), "spec", "chart", "version")

	return doc, nil
}
