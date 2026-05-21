package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dmgk/modules2tuple/v2/config"
)

type GithubCommit struct {
	SHA string `json:"sha"`
}

type GithubRef struct {
	Ref string `json:"ref"`
}

var githubRateLimitError = fmt.Sprintf(`Github API rate limit exceeded. Please either:
- set %s environment variable to your Github "username:personal_access_token"
  to let modules2tuple call Github API using basic authentication.
  To create a new token, navigate to https://github.com/settings/tokens/new
  (leave all checkboxes unchecked, modules2tuple doesn't need any access to your account)
- set %s=1 or pass "-offline" flag to module2tuple to disable network access`,
	config.GithubCredentialsKey, config.OfflineKey)

func GithubGetCommit(account, project, tag string) (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits/%s", url.PathEscape(account), url.PathEscape(project), tag)

	resp, err := get(url, config.GithubUsername, config.GithubToken)
	if err != nil {
		if strings.Contains(err.Error(), "API rate limit exceeded") {
			return "", errors.New(githubRateLimitError)
		}
		return "", fmt.Errorf("error getting commit %s for %s/%s: %v", tag, account, project, err)
	}

	var res GithubCommit
	if err := json.Unmarshal(resp, &res); err != nil {
		return "", fmt.Errorf("error unmarshalling: %v, resp: %v", err, string(resp))
	}

	return res.SHA, nil
}

func GithubHasTag(account, project, tag string) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/tags/%s", url.PathEscape(account), url.PathEscape(project), tag)

	resp, err := get(url, config.GithubUsername, config.GithubToken)
	if err != nil {
		if err == errNotFound {
			return false, nil
		}
		if strings.Contains(err.Error(), "API rate limit exceeded") {
			return false, errors.New(githubRateLimitError)
		}
		return false, fmt.Errorf("error getting refs for %s/%s: %v", account, project, err)
	}

	var ref GithubRef
	if err := json.Unmarshal(resp, &ref); err != nil {
		switch err := err.(type) {
		case *json.UnmarshalTypeError:
			// type mismatch during unmarshal, tag was incomplete and the API returned an array
			return false, nil
		default:
			return false, fmt.Errorf("error unmarshalling: %v, resp: %v", err, string(resp))
		}
	}

	return true, nil
}

func GithubListTags(account, project, prefix string) ([]string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/tags/%s", url.PathEscape(account), url.PathEscape(project), url.PathEscape(prefix))

	resp, err := get(url, config.GithubUsername, config.GithubToken)
	if err != nil {
		if strings.Contains(err.Error(), "API rate limit exceeded") {
			return nil, errors.New(githubRateLimitError)
		}
		return nil, fmt.Errorf("error getting refs for %s/%s: %v", account, project, err)
	}

	var refs []GithubRef
	if err := json.Unmarshal(resp, &refs); err != nil {
		if _, ok := err.(*json.UnmarshalTypeError); ok {
			// type mismatch: prefix matched an exact tag, so the API returned a single
			// object instead of an array. Decode it and return as a one-element slice.
			var ref GithubRef
			if err := json.Unmarshal(resp, &ref); err != nil {
				return nil, fmt.Errorf("error unmarshalling: %v, resp: %v", err, string(resp))
			}
			return []string{ref.Ref}, nil
		}
		return nil, fmt.Errorf("error unmarshalling: %v, resp: %v", err, string(resp))
	}

	var res []string
	for _, r := range refs {
		res = append(res, r.Ref)
	}

	return res, nil
}

var majorVersionSuffixRe = regexp.MustCompile(`/v[2-9]\d*$`)

func GithubLookupTag(account, project, path, tag string) (string, error) {
	// For submodules, prefer the path-prefixed tag form (e.g. "featuregate/v1.9.0"):
	// multi-module repos often also tag the root at the same version (e.g. "v1.9.0"),
	// which would otherwise be picked up and point at the wrong tree.
	//
	// When the path ends in a Go "/vN" major-version suffix (e.g. ".../endpoints/v2"),
	// strip it: the real tag is "endpoints/v2.6.9", not "endpoints/v2/v2.6.9".
	lookupPath := majorVersionSuffixRe.ReplaceAllString(path, "")
	if lookupPath != "" {
		allTags, err := GithubListTags(account, project, lookupPath)
		if err != nil {
			return "", err
		}
		// Github API returns tags sorted by creation time, earliest first.
		// Iterate through them in reverse order to find the most recent matching tag.
		for i := len(allTags) - 1; i >= 0; i-- {
			if strings.HasSuffix(allTags[i], filepath.Join(lookupPath, tag)) {
				return strings.TrimPrefix(allTags[i], "refs/tags/"), nil
			}
		}
	}

	hasTag, err := GithubHasTag(account, project, tag)
	if err != nil {
		return "", err
	}
	if hasTag {
		return tag, nil
	}

	return "", fmt.Errorf("tag %v doesn't seem to exist in %s/%s", tag, account, project)
}

func GithubHasContentsAtPath(account, project, path, tag string) (bool, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", url.PathEscape(account), url.PathEscape(project), path, tag)

	// Ignore response, we care only about errors
	_, err := get(url, config.GithubUsername, config.GithubToken)
	if err != nil && err != errNotFound {
		return false, err
	}
	return err == nil, nil
}
