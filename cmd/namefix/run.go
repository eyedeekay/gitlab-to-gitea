package main

import (
	"fmt"
	"time"

	"github.com/go-i2p/gitlab-to-gitea/utils"
)

// Run performs the repository name inconsistency check
func (nf *NameFixer) Run() ([]Inconsistency, error) {
	// Step 1: Fetch repositories for each owner directly
	sourceRepos, err := nf.getOwnerRepositories(nf.sourceOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repositories for %s: %w", nf.sourceOwner, err)
	}
	utils.PrintInfo(fmt.Sprintf("Found %d repositories owned by %s", len(sourceRepos), nf.sourceOwner))

	targetRepos, err := nf.getOwnerRepositories(nf.targetOwner)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repositories for %s: %w", nf.targetOwner, err)
	}
	utils.PrintInfo(fmt.Sprintf("Found %d repositories owned by %s", len(targetRepos), nf.targetOwner))

	if len(sourceRepos) == 0 || len(targetRepos) == 0 {
		utils.PrintWarning("Not enough repositories to compare. Ensure both owners have repositories.")
		return []Inconsistency{}, nil
	}

	// Step 3: Get initial commit hashes for source repositories
	utils.PrintInfo("Extracting initial commit hashes for source repositories...")
	sourceReposWithCommits := make([]Repository, 0, len(sourceRepos))
	for _, repo := range sourceRepos {
		repoName, _ := repo["name"].(string)
		initialCommit, err := nf.getInitialCommit(nf.sourceOwner, repoName)
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Could not get initial commit for %s/%s: %v", nf.sourceOwner, repoName, err))
			continue
		}

		// Debug output to verify hash extraction
		utils.PrintInfo(fmt.Sprintf("Source repo %s/%s has initial commit: %s",
			nf.sourceOwner, repoName, initialCommit))

		sourceReposWithCommits = append(sourceReposWithCommits, Repository{
			Owner:             nf.sourceOwner,
			Name:              repoName,
			InitialCommitHash: initialCommit,
		})

		// Add a small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	// Step 4: Get initial commit hashes for target repositories
	utils.PrintInfo("Extracting initial commit hashes for target repositories...")
	targetReposWithCommits := make([]Repository, 0, len(targetRepos))
	for _, repo := range targetRepos {
		repoName, _ := repo["name"].(string)
		initialCommit, err := nf.getInitialCommit(nf.targetOwner, repoName)
		if err != nil {
			utils.PrintWarning(fmt.Sprintf("Could not get initial commit for %s/%s: %v", nf.targetOwner, repoName, err))
			continue
		}

		// Debug output to verify hash extraction
		utils.PrintInfo(fmt.Sprintf("Target repo %s/%s has initial commit: %s",
			nf.targetOwner, repoName, initialCommit))

		targetReposWithCommits = append(targetReposWithCommits, Repository{
			Owner:             nf.targetOwner,
			Name:              repoName,
			InitialCommitHash: initialCommit,
		})

		// Add a small delay to avoid rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	// Step 5: Compare repositories and find inconsistencies
	utils.PrintInfo("Comparing repositories to find naming inconsistencies...")
	var inconsistencies []Inconsistency
	var matchesFound int

	// For each target repository, check if it matches any source repository by commit hash
	for _, targetRepo := range targetReposWithCommits {
		foundMatch := false

		for _, sourceRepo := range sourceReposWithCommits {
			// Check if initial commits match (indicating a fork relationship)
			if targetRepo.InitialCommitHash == sourceRepo.InitialCommitHash {
				foundMatch = true
				matchesFound++

				// Debug output showing hash match
				utils.PrintInfo(fmt.Sprintf("MATCH FOUND: %s/%s and %s/%s have the same initial commit: %s",
					sourceRepo.Owner, sourceRepo.Name,
					targetRepo.Owner, targetRepo.Name,
					targetRepo.InitialCommitHash))

				// Check if the repository names are different
				if targetRepo.Name != sourceRepo.Name {
					utils.PrintInfo(fmt.Sprintf("INCONSISTENCY: %s/%s -> %s/%s",
						sourceRepo.Owner, sourceRepo.Name,
						targetRepo.Owner, targetRepo.Name))

					inconsistencies = append(inconsistencies, Inconsistency{
						SourceRepo: sourceRepo,
						TargetRepo: targetRepo,
					})
				}

				// We found a match, no need to check other source repos
				break
			}
		}

		if foundMatch {
			utils.PrintInfo(fmt.Sprintf("Match found for %s/%s with hash %s",
				targetRepo.Owner, targetRepo.Name,
				targetRepo.InitialCommitHash))
		}
	}

	utils.PrintInfo(fmt.Sprintf("Total commit hash matches found: %d", matchesFound))
	utils.PrintInfo(fmt.Sprintf("Naming inconsistencies detected: %d", len(inconsistencies)))

	return inconsistencies, nil
}
