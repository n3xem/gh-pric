package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"git.pepabo.com/yukyan/gh-pric/github/model"
	"github.com/cli/go-gh/v2/pkg/api"
)

// Client は GitHub API を操作するためのクライアント
type Client struct {
	client *api.RESTClient
}

// NewClient は新しいGitHubクライアントを作成します
func NewClient() (*Client, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize GitHub client: %w", err)
	}
	
	return &Client{
		client: client,
	}, nil
}

// GetUsername は現在認証されているユーザー名を取得します
func (c *Client) GetUsername() (string, error) {
	userInfo := struct {
		Login string `json:"login"`
	}{}
	
	err := c.client.Get("user", &userInfo)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve user information: %w", err)
	}
	
	return userInfo.Login, nil
}

// FetchIssues はGitHub APIからIssueを取得します
func (c *Client) FetchIssues(ctx context.Context, username, involvement string, dateRange model.DateRange) ([]model.Item, error) {
	// Query parameters for filtering by date range
	startDateStr := dateRange.StartDate.Format("2006-01-02")
	
	// Construct appropriate query parameters based on involvement
	var query string
	switch involvement {
	case "created":
		query = fmt.Sprintf("search/issues?q=is:issue+author:%s+created:>=%s&per_page=100", 
			username, startDateStr)
	case "assigned":
		query = fmt.Sprintf("search/issues?q=is:issue+assignee:%s+created:>=%s&per_page=100", 
			username, startDateStr)
	case "commented":
		query = fmt.Sprintf("search/issues?q=is:issue+commenter:%s+created:>=%s&per_page=100", 
			username, startDateStr)
	default:
		query = fmt.Sprintf("search/issues?q=is:issue+involves:%s+created:>=%s&per_page=100", 
			username, startDateStr)
	}
	
	items := []model.Item{}
	page := 1
	hasMore := true

	for hasMore {
		var response struct {
			Items []struct {
				URL           string    `json:"html_url"`
				Number        int       `json:"number"`
				Title         string    `json:"title"`
				State         string    `json:"state"`
				CreatedAt     time.Time `json:"created_at"`
				UpdatedAt     time.Time `json:"updated_at"`
				RepositoryURL string    `json:"repository_url"`
				User          struct {
					Login string `json:"login"`
				} `json:"user"`
				Assignees []struct {
					Login string `json:"login"`
				} `json:"assignees"`
				Labels []struct {
					Name string `json:"name"`
				} `json:"labels"`
			} `json:"items"`
		}
		
		pageQuery := fmt.Sprintf("%s&page=%d", query, page)
		
		// Add retry functionality
		var err error
		maxRetries := 3
		for retryCount := 0; retryCount < maxRetries; retryCount++ {
			err = c.client.Get(pageQuery, &response)
			if err == nil {
				break
			}
			
			// Wait before retrying
			time.Sleep(2 * time.Second)
		}
		
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve Issues: %w", err)
		}
		
		// Exit if the response is empty
		if len(response.Items) == 0 {
			hasMore = false
			continue
		}

		for _, issue := range response.Items {
			// Skip items outside the date range
			if issue.CreatedAt.After(dateRange.EndDate) || issue.CreatedAt.Before(dateRange.StartDate) {
				continue
			}

			// Extract repository name
			repoURL := issue.RepositoryURL
			repoParts := strings.Split(repoURL, "/")
			repoName := ""
			if len(repoParts) >= 2 {
				repoName = fmt.Sprintf("%s/%s", repoParts[len(repoParts)-2], repoParts[len(repoParts)-1])
			}

			// Extract assignees
			assignees := make([]string, len(issue.Assignees))
			for i, a := range issue.Assignees {
				assignees[i] = a.Login
			}

			// Extract labels
			labels := make([]string, len(issue.Labels))
			for i, l := range issue.Labels {
				labels[i] = l.Name
			}

			item := model.Item{
				Type:       "Issue",
				Number:     issue.Number,
				Title:      issue.Title,
				URL:        issue.URL,
				State:      issue.State,
				CreatedAt:  issue.CreatedAt,
				UpdatedAt:  issue.UpdatedAt,
				Author:     issue.User.Login,
				Assignees:  assignees,
				Labels:     labels,
				Repository: repoName,
			}
			items = append(items, item)
		}

		// Consider Rate Limit
		time.Sleep(1 * time.Second)
		page++
		
		// Exit if a certain number has been retrieved (optional)
		if page > 10 {
			hasMore = false
		}
	}

	return items, nil
}

// FetchPRs はGitHub APIからPRを取得します
func (c *Client) FetchPRs(ctx context.Context, username, involvement string, dateRange model.DateRange) ([]model.Item, error) {
	// Query parameters for filtering by date range
	startDateStr := dateRange.StartDate.Format("2006-01-02")
	
	query := fmt.Sprintf("search/issues?q=is:pr+%s:%s+created:>=%s&per_page=100", 
		getInvolvementQuery(involvement), username, startDateStr)
	
	items := []model.Item{}
	page := 1
	hasMore := true

	for hasMore {
		var response struct {
			Items []struct {
				URL           string    `json:"html_url"`
				Number        int       `json:"number"`
				Title         string    `json:"title"`
				State         string    `json:"state"`
				CreatedAt     time.Time `json:"created_at"`
				UpdatedAt     time.Time `json:"updated_at"`
				RepositoryURL string    `json:"repository_url"`
				User          struct {
					Login string `json:"login"`
				} `json:"user"`
				Assignees []struct {
					Login string `json:"login"`
				} `json:"assignees"`
				Labels []struct {
					Name string `json:"name"`
				} `json:"labels"`
				PullRequest struct {
					URL string `json:"url"`
				} `json:"pull_request"`
			} `json:"items"`
		}
		
		pageQuery := fmt.Sprintf("%s&page=%d", query, page)
		
		// Add retry functionality
		var err error
		maxRetries := 3
		for retryCount := 0; retryCount < maxRetries; retryCount++ {
			err = c.client.Get(pageQuery, &response)
			if err == nil {
				break
			}
			
			// Wait before retrying
			time.Sleep(2 * time.Second)
		}
		
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve PRs: %w", err)
		}
		
		// Exit if the response is empty
		if len(response.Items) == 0 {
			hasMore = false
			continue
		}

		for _, pr := range response.Items {
			// Skip items outside the date range
			if pr.CreatedAt.After(dateRange.EndDate) || pr.CreatedAt.Before(dateRange.StartDate) {
				continue
			}

			// Extract repository name
			repoURL := pr.RepositoryURL
			repoParts := strings.Split(repoURL, "/")
			repoName := ""
			if len(repoParts) >= 2 {
				repoName = fmt.Sprintf("%s/%s", repoParts[len(repoParts)-2], repoParts[len(repoParts)-1])
			}

			// Extract assignees
			assignees := make([]string, len(pr.Assignees))
			for i, a := range pr.Assignees {
				assignees[i] = a.Login
			}

			// Extract labels
			labels := make([]string, len(pr.Labels))
			for i, l := range pr.Labels {
				labels[i] = l.Name
			}

			item := model.Item{
				Type:       "PR",
				Number:     pr.Number,
				Title:      pr.Title,
				URL:        pr.URL,
				State:      pr.State,
				CreatedAt:  pr.CreatedAt,
				UpdatedAt:  pr.UpdatedAt,
				Author:     pr.User.Login,
				Assignees:  assignees,
				Labels:     labels,
				Repository: repoName,
			}
			items = append(items, item)
		}

		// Consider Rate Limit
		time.Sleep(1 * time.Second)
		page++
		
		// Exit if a certain number has been retrieved (optional)
		if page > 10 {
			hasMore = false
		}
	}

	return items, nil
}

// FetchIssueDetails はIssueの詳細情報（本文やコメント）を取得します
func (c *Client) FetchIssueDetails(ctx context.Context, item *model.Item) error {
	// Extract repository name and Issue number
	repoPath := getRepoPathFromURL(item.Repository)
	if repoPath == "" {
		return fmt.Errorf("Failed to extract repository path: %s", item.Repository)
	}
	
	// Retrieve Issue details
	var issueDetail struct {
		Body string `json:"body"`
	}
	
	issueURL := fmt.Sprintf("repos/%s/issues/%d", repoPath, item.Number)
	
	// Use retry functionality
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = c.client.Get(issueURL, &issueDetail)
		if err == nil {
			break
		}
		
		// Wait before retrying
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("Failed to retrieve Issue details: %w", err)
	}
	
	item.Body = issueDetail.Body
	
	// Retrieve comments
	return c.FetchComments(ctx, item, fmt.Sprintf("repos/%s/issues/%d/comments", repoPath, item.Number))
}

// FetchPRDetails はPRの詳細情報（本文やコメント）を取得します
func (c *Client) FetchPRDetails(ctx context.Context, item *model.Item) error {
	// Extract repository name and PR number
	repoPath := getRepoPathFromURL(item.Repository)
	if repoPath == "" {
		return fmt.Errorf("Failed to extract repository path: %s", item.Repository)
	}
	
	// Retrieve PR details (PR can also be retrieved from the Issue endpoint)
	var prDetail struct {
		Body string `json:"body"`
	}
	
	prURL := fmt.Sprintf("repos/%s/pulls/%d", repoPath, item.Number)
	
	// Use retry functionality
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = c.client.Get(prURL, &prDetail)
		if err == nil {
			break
		}
		
		// Wait before retrying
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("Failed to retrieve PR details: %w", err)
	}
	
	item.Body = prDetail.Body
	
	// Retrieve comments
	issueCommentsURL := fmt.Sprintf("repos/%s/issues/%d/comments", repoPath, item.Number)
	err = c.FetchComments(ctx, item, issueCommentsURL)
	if err != nil {
		return err
	}
	
	// Also retrieve PR review comments
	reviewCommentsURL := fmt.Sprintf("repos/%s/pulls/%d/comments", repoPath, item.Number)
	return c.FetchReviewComments(ctx, item, reviewCommentsURL)
}

// FetchComments はコメントを取得します
func (c *Client) FetchComments(ctx context.Context, item *model.Item, commentsURL string) error {
	var comments []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	
	// Use retry functionality
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = c.client.Get(commentsURL, &comments)
		if err == nil {
			break
		}
		
		// Wait before retrying
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("Failed to retrieve comments: %w", err)
	}
	
	// Add comments to the Item struct
	for _, c := range comments {
		item.Comments = append(item.Comments, model.Comment{
			Author:    c.User.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	
	return nil
}

// FetchReviewComments はPRのレビューコメントを取得します
func (c *Client) FetchReviewComments(ctx context.Context, item *model.Item, reviewCommentsURL string) error {
	var reviewComments []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	
	// Use retry functionality
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = c.client.Get(reviewCommentsURL, &reviewComments)
		if err == nil {
			break
		}
		
		// Wait before retrying
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("Failed to retrieve review comments: %w", err)
	}
	
	// Add review comments to the Item struct
	for _, rc := range reviewComments {
		item.Comments = append(item.Comments, model.Comment{
			Author:    rc.User.Login,
			Body:      rc.Body,
			CreatedAt: rc.CreatedAt,
			UpdatedAt: rc.UpdatedAt,
		})
	}
	
	return nil
}

// FilterIgnoredUserComments は特定のユーザーからのコメントを除外します
func FilterIgnoredUserComments(items []model.Item, ignoreUsers []string) {
	for i := range items {
		var filteredComments []model.Comment
		for _, comment := range items[i].Comments {
			// Keep the comment if the user is not in the ignoreUsers list
			shouldKeep := true
			for _, ignoreUser := range ignoreUsers {
				if comment.Author == ignoreUser {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				filteredComments = append(filteredComments, comment)
			}
		}
		items[i].Comments = filteredComments
	}
}

// GitHubクエリのインボルブメントタイプを取得します
func getInvolvementQuery(involvement string) string {
	switch involvement {
	case "created":
		return "author"
	case "assigned":
		return "assignee"
	case "reviewed":
		return "reviewed-by"
	case "commented":
		return "commenter"
	default:
		return "involves"
	}
}

// URLからリポジトリパスを抽出します
func getRepoPathFromURL(repoURL string) string {
	// First, check the repository URL format
	if strings.HasPrefix(repoURL, "http") {
		// Extract the path from the URL (e.g., https://github.com/owner/repo → owner/repo)
		u, err := url.Parse(repoURL)
		if err != nil {
			return ""
		}
		path := strings.TrimPrefix(u.Path, "/")
		return path
	} else if strings.Contains(repoURL, "/") {
		// If it's already in owner/repo format, return it as is
		return repoURL
	}
	
	return ""
} 
