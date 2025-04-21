// This is different, and worse, than everything else in this repo.
// It is a script to delete users and groups from a GitLab instance.
// It still contains I2P-specific behavior.
// It does not use the official gitlab API client for go, it uses a custom one which might be janky.
// I probably won't fix it.

// I SERIOUSLY DON'T THINK THIS IS A GOOD IDEA FOR YOU TO USE.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
)

const (
	disclaimerMessage = `
# This is a gitlab anti-bot script for selfhosted instances.
# It was used on i2pgit.org after a bot attack.
# You **WILL** need to modify it for other instances.
# You **WILL** need to set up an API key with very dangerous perms to use it.
# It **WILL** break your shit in terrible, unfixable ways if you aren't very very careful.
# You **SHOULD NEVER RUN THIS SCRIPT WITHOUT READING IT TWICE FIRST**
# FOR REAL this is like a BIG RED BUTTON UNDER A BULLETPROOF GLASS CASE WITH TWO KEYS.
# IN CASE OF BOT ARMY consider NOT using this thing but maybe if you really have to, then ask for help, then use it.
# IT'S WEEDEATING WITH A FLAMETHROWER. IT'S CHAINSAW SURGERY. I CANNOT EMPHASIZE THAT ENOUGH.
# I HAVE NO DIRECT KNOWLEDGE OF IT KILLING ANY KITTENS BUT I CANNOT RULE IT OUT.
# That said, it did take care of the bot problem.
`
)

var (
	gitlabAPIURL = "https://gitlab.example.com/api/v4" // Modify to your GitLab instance
	gitlabToken  = ""                                  // Set your API token here or via env var
)

type User struct {
	ID               int         `json:"id"`
	LastActivityOn   interface{} `json:"last_activity_on"` // Could be null
	IsFollowed       bool        `json:"is_followed"`
	CanCreateProject bool        `json:"can_create_project"`
}

type Membership struct {
	SourceType string `json:"source_type"`
	SourceName string `json:"source_name"`
	SourceID   int    `json:"source_id"`
}

func init() {
	// Check for token in env var
	if envToken := os.Getenv("GITLAB_API_TOKEN"); envToken != "" {
		gitlabToken = envToken
	}

	if gitlabToken == "" {
		fmt.Println("ERROR: GITLAB_API_TOKEN not set. Please set it as an environment variable or in the code.")
		os.Exit(1)
	}

	// Display disclaimer
	fmt.Println(disclaimerMessage)
	fmt.Println("Are you ABSOLUTELY SURE you want to continue? This can cause IRREVERSIBLE DAMAGE.")
	fmt.Print("Type 'YES I UNDERSTAND THE CONSEQUENCES' to continue: ")

	var confirmation string
	fmt.Scanln(&confirmation)
	if confirmation != "YES I UNDERSTAND THE CONSEQUENCES" {
		fmt.Println("Aborted.")
		os.Exit(0)
	}
}

func main() {
	users, err := getAllUsers()
	if err != nil {
		fmt.Printf("Error getting users: %v\n", err)
		os.Exit(1)
	}

	for _, user := range users {
		fmt.Println("begin loop")
		fmt.Printf("User ID: %d\n", user.ID)

		if user.IsFollowed {
			fmt.Println("User is followed, skipping")
			continue
		}

		if user.LastActivityOn == nil {
			fmt.Printf("User %d has no activity, deleting\n", user.ID)
			deleteUser(user.ID)
			continue
		}

		if !user.CanCreateProject {
			fmt.Printf("User %d cannot create projects, deleting\n", user.ID)
			deleteUser(user.ID)
			continue
		}

		memberships, err := getUserMemberships(user.ID)
		if err != nil {
			fmt.Printf("Error getting memberships for user %d: %v\n", user.ID, err)
			continue
		}

		if len(memberships) == 0 {
			fmt.Printf("User %d has no memberships, deleting\n", user.ID)
			deleteUser(user.ID)
			continue
		}

		hasI2PProject := false
		for _, membership := range memberships {
			fmt.Printf("Project is: %s\n", membership.SourceType)

			if membership.SourceType == "Project" {
				fmt.Printf("Project name is: %s\n", membership.SourceName)

				if isI2PRelated(membership.SourceName) {
					fmt.Printf("%s contained an I2P related term\n", membership.SourceName)
					hasI2PProject = true
					break
				}
			}

			if membership.SourceType == "Namespace" {
				fmt.Printf("Group name is: %s\n", membership.SourceName)

				if !isI2PRelated(membership.SourceName) {
					fmt.Printf("Deleting non-I2P related group %d\n", membership.SourceID)
					deleteGroup(membership.SourceID)
				}
			}
		}

		if hasI2PProject {
			fmt.Printf("User %d has I2P related project.\n", user.ID)
		} else {
			fmt.Printf("User %d has no I2P related projects, deleting\n", user.ID)
			deleteUser(user.ID)
		}

		fmt.Println("")
	}
}

func isI2PRelated(name string) bool {
	return regexp.MustCompile(`(?i)i2p`).MatchString(name)
}

func getAllUsers() ([]User, error) {
	var allUsers []User
	page := 1

	for {
		url := fmt.Sprintf("%s/users?page=%d&per_page=100", gitlabAPIURL, page)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("PRIVATE-TOKEN", gitlabToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("API returned status code %d", resp.StatusCode)
		}

		var users []User
		if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
			return nil, err
		}

		if len(users) == 0 {
			break
		}

		allUsers = append(allUsers, users...)
		page++
	}

	return allUsers, nil
}

func getUserMemberships(userID int) ([]Membership, error) {
	url := fmt.Sprintf("%s/users/%d/memberships", gitlabAPIURL, userID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("PRIVATE-TOKEN", gitlabToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status code %d: %s", resp.StatusCode, string(body))
	}

	var memberships []Membership
	if err := json.NewDecoder(resp.Body).Decode(&memberships); err != nil {
		return nil, err
	}

	return memberships, nil
}

func deleteUser(userID int) {
	url := fmt.Sprintf("%s/users/%d", gitlabAPIURL, userID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Printf("Error creating delete request for user %d: %v\n", userID, err)
		return
	}

	req.Header.Set("PRIVATE-TOKEN", gitlabToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error deleting user %d: %v\n", userID, err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("Successfully deleted user %d\n", userID)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to delete user %d. Status: %d, Response: %s\n", userID, resp.StatusCode, string(body))
	}
}

func deleteGroup(groupID int) {
	url := fmt.Sprintf("%s/groups/%d", gitlabAPIURL, groupID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		fmt.Printf("Error creating delete request for group %d: %v\n", groupID, err)
		return
	}

	req.Header.Set("PRIVATE-TOKEN", gitlabToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Error deleting group %d: %v\n", groupID, err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		fmt.Printf("Successfully deleted group %d\n", groupID)
	} else {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to delete group %d. Status: %d, Response: %s\n", groupID, resp.StatusCode, string(body))
	}
}
