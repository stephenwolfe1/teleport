/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package bot

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
)

// Assign will assign reviewers for this PR.
//
// Assign works by parsing the PR, discovering the changes, and returning a
// set of reviewers determined by: content of the PR, if the author is internal
// or external, and team they are on.
func (b *Bot) Assign(ctx context.Context) error {
	var err error
	var reviewers []string

	switch {
	// If this PR is a backport PR try and assign original reviewers. If the
	// original reviewers can not be found, then put it through the normal
	// review process.
	case isBackport(b.c.Environment.UnsafeBase):
		reviewers, err = b.getBackportReviewers(ctx)
		if err != nil {
			reviewers, err = b.getReviewers(ctx)
		}
	default:
		reviewers, err := b.getReviewers(ctx)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	log.Printf("Assign: Requesting reviews from: %v.", reviewers)

	// Request GitHub assign reviewers to this PR.
	err = b.c.GitHub.RequestReviewers(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number,
		reviewers)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (b *Bot) getReviewers(ctx context.Context) ([]string, error) {
	docs, code, err := b.parseChanges(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return b.c.Review.Get(b.c.Environment.Author, docs, code), nil
}

func (b *Bot) getBackportReviewers(ctx context.Context) ([]string, error) {
	originalNumber, err := b.parseOriginal(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		b.c.Environment.Number)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reviews, err := b.c.GitHub.ListReviews(ctx,
		b.c.Environment.Organization,
		b.c.Environment.Repository,
		originalNumber)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var reviewers []string
	for _, review := range reviews {
		if review.State != "APPROVED" {
			continue
		}

		reviewers = append(reviewers, review.Author)
	}
	if len(reviewers) < 2 {
		return nil, trace.IsNotFound("invalid")
	}

	return reviewers, nil
}

func (b *Bot) parseOriginal(ctx context.Context, organization string, repository string, number int) (int, error) {
	pull, err := b.c.GitHub.Get(ctx,
		organization,
		repository,
		number)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	// Search inside both the title and body.
	matches := pattern.FindAllStringSubmatch(pull.Title+pull.Body, -1)
	if len(matches) != 1 {
		return trace.BadParameter("found multiple matches, unable to find original")
	}

	number, err := strconv.Atoi(matches[0])
	if err != nil {
		return trace.Wrap(err)
	}

	return number, nil
}

func isBackport(unsafeBase string) bool {
	if strings.HasPrefix(unsafeBase, "branch/v") {
		return true
	}
	return false
}

var pattern = regexp.MustCompile(`(?m)#[0-9]+`)
