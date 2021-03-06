package main

import (
	"bytes"
	"fmt"
	"github.com/juju/errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/github"
)

var repoQuery string

var allSQLInfraMembers []string
var allSQLInfraMemberEmals []string

var github2Email map[string]string

const (
	// go-github/github incorrectly handles URL escape with "+", so we avoid "+" by using a UTC time
	githubUTCDateFormat = "2006-01-02T15:04:05Z"
)

// IssueSlice is the slice of issues
type IssueSlice []github.Issue

func (s IssueSlice) Len() int      { return len(s) }
func (s IssueSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s IssueSlice) Less(i, j int) bool {
	return s[i].GetHTMLURL() < s[j].GetHTMLURL()
}

func getIssuesByQuery(bySort string, queryArg string) IssueSlice {
	opt := github.SearchOptions{
		Sort: bySort,
	}

	var allIssues IssueSlice

	query := bytes.NewBufferString(repoQuery)
	query.WriteString(queryArg)

	retryCount := 0
	for {
		issues, resp, err := githubClient.Search.Issues(globalCtx, query.String(), &opt)
		if err1, ok := err.(*github.RateLimitError); ok {
			dur := err1.Rate.Reset.Time.Sub(time.Now())
			if dur < 0 {
				dur = time.Minute
			}
			retryCount++
			if retryCount <= 10 {
				fmt.Printf("meet RateLimitError, wait %s and retry %d\n", dur, retryCount)
				time.Sleep(dur)
				continue
			}
		}

		perror(errors.Trace(err))

		allIssues = append(allIssues, issues.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	sort.Sort(allIssues)
	return allIssues
}

func getIssues(bySort string, queryArgs map[string]string) IssueSlice {
	query := bytes.NewBufferString("")

	for key, value := range queryArgs {
		query.WriteString(fmt.Sprintf(" %s:%s", key, value))
	}

	return getIssuesByQuery(bySort, query.String())
}

func generateDateRangeQuery(start string, end *string) string {
	if end != nil {
		return fmt.Sprintf("%s..%s", start, *end)
	} else {
		return fmt.Sprintf(">=%s", start)
	}
}

func getCreatedIssues(start string, end *string) []github.Issue {
	return getIssues("created", map[string]string{
		"is":      "issue",
		"created": generateDateRangeQuery(start, end),
	})
}

func getCreatedPullRequests(start string, end *string) []github.Issue {
	return getIssues("created", map[string]string{
		"is":      "pr",
		"created": generateDateRangeQuery(start, end),
	})
}

func getPullReuestsMentioned(start string, end *string, mentions string) []github.Issue {
	return getIssues("updated", map[string]string{
		"is":       "pr",
		"mentions": mentions,
		"-author":  mentions,
		"updated":  generateDateRangeQuery(start, end),
	})
}

func getReviewPullRequests(user string, start string, end *string) []github.Issue {
	return getIssues("updated", map[string]string{
		"is":        "open",
		"type":      "pr",
		"commenter": user,
		"-author":   user,
		"updated":   generateDateRangeQuery(start, end),
	})
}

func initRepoQuery() {
	s := strings.Join(config.Github.Repos, " repo:")
	repoQuery = "repo:" + s
}

func initTeamMembers() {
	github2Email = make(map[string]string)
	for _, team := range config.Teams {
		for _, member := range team.Members {
			if team.Name == "SQL-Infra" {
				allSQLInfraMembers = append(allSQLInfraMembers, member.Github)
				allSQLInfraMemberEmals = append(allSQLInfraMemberEmals, strconv.Quote(member.Email))
			}

			github2Email[member.Github] = member.Email
		}
	}
}
