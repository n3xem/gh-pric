package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Struct for setting the date range
type DateRange struct {
	StartDate time.Time
	EndDate   time.Time
}

// Struct to hold information about PRs and Issues
type Item struct {
	Type        string    // "PR" or "Issue"
	Number      int       // PR number or Issue number
	Title       string    // Title
	URL         string    // URL
	State       string    // State (open, closed, merged)
	CreatedAt   time.Time // Creation date
	UpdatedAt   time.Time // Update date
	Author      string    // Author
	Assignees   []string  // Assignees
	Labels      []string  // Labels
	Repository  string    // Repository name
	Involvement string    // Involvement type (created, assigned, commented)
	Body        string    // Body
	Comments    []Comment // Comments
}

// Struct to hold comment information
type Comment struct {
	Author    string    // Comment author
	Body      string    // Comment body
	CreatedAt time.Time // Date of posting
	UpdatedAt time.Time // Update date
}

func main() {
	// Command line argument parsing
	var startDateStr, endDateStr, outputFile string
	var commentIgnoreUsers string
	var outputFormat string
	var defaultEndDate = time.Now().Format("2006-01-02")
	var defaultStartDate = time.Now().AddDate(0, 0, -3).Format("2006-01-02") // Default is 3 days ago

	flag.StringVar(&startDateStr, "from", defaultStartDate, "Start date (YYYY-MM-DD format)")
	flag.StringVar(&endDateStr, "to", defaultEndDate, "End date (YYYY-MM-DD format)")
	flag.StringVar(&outputFile, "output", "github-activity.txt", "Output file name")
	flag.StringVar(&outputFile, "o", "github-activity.txt", "Output file name (alias for --output)")
	flag.StringVar(&commentIgnoreUsers, "comment-ignore", "", "Usernames of comments to exclude from output (comma-separated for multiple)")
	flag.StringVar(&outputFormat, "output-format", "md", "Output format (md or json)")
	flag.Parse()

	// Output format validation
	if outputFormat != "md" && outputFormat != "json" {
		fmt.Fprintf(os.Stderr, "Invalid output format: %s (please specify md or json)\n", outputFormat)
		os.Exit(1)
	}

	// Create a list of users to ignore for comments
	var ignoreUsers []string
	if commentIgnoreUsers != "" {
		ignoreUsers = strings.Split(commentIgnoreUsers, ",")
		for i, user := range ignoreUsers {
			ignoreUsers[i] = strings.TrimSpace(user)
		}
	}

	// Parse dates
	dateRange, err := parseDateRange(startDateStr, endDateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse dates: %v\n", err)
		os.Exit(1)
	}

	// Initialize GitHub client
	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize GitHub client: %v\n", err)
		os.Exit(1)
	}

	// Retrieve user information
	userInfo := struct {
		Login string `json:"login"`
	}{}
	err = client.Get("user", &userInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to retrieve user information: %v\n", err)
		os.Exit(1)
	}

	username := userInfo.Login
	fmt.Printf("Retrieving GitHub activity for user '%s'...\n", username)
	fmt.Printf("Period: %s to %s\n", dateRange.StartDate.Format("2006-01-02"), dateRange.EndDate.Format("2006-01-02"))

	// Data retrieval
	items, err := fetchAllItems(client, username, dateRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to retrieve data: %v\n", err)
		os.Exit(1)
	}

	// Filter comments from specific users
	if len(ignoreUsers) > 0 {
		filterIgnoredUserComments(items, ignoreUsers)
	}

	// Output results
	err = writeResultsToFile(items, outputFile, username, dateRange, outputFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Results saved to %s\n", outputFile)
}

// Parse date strings and return the date range
func parseDateRange(startStr, endStr string) (DateRange, error) {
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return DateRange{}, fmt.Errorf("Failed to parse start date: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return DateRange{}, fmt.Errorf("Failed to parse end date: %w", err)
	}

	// Set end date to 23:59:59
	endDate = endDate.Add(24*time.Hour - time.Second)

	if endDate.Before(startDate) {
		return DateRange{}, fmt.Errorf("End date must be after start date")
	}

	return DateRange{
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

// Retrieve PRs and Issues related to the user from the GitHub API
func fetchAllItems(client *api.RESTClient, username string, dateRange DateRange) ([]Item, error) {
	var allItems []Item
	ctx := context.Background()

	// Retrieve created Issues
	createdIssues, err := fetchIssues(client, ctx, username, "created", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range createdIssues {
		createdIssues[i].Involvement = "created"
		// Retrieve Issue details (body and comments)
		err = fetchIssueDetails(client, ctx, &createdIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for Issue (ID: %d): %v\n", createdIssues[i].Number, err)
		}
	}
	allItems = append(allItems, createdIssues...)

	// Retrieve assigned Issues
	assignedIssues, err := fetchIssues(client, ctx, username, "assigned", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range assignedIssues {
		assignedIssues[i].Involvement = "assigned"
		// Retrieve Issue details (body and comments)
		err = fetchIssueDetails(client, ctx, &assignedIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for Issue (ID: %d): %v\n", assignedIssues[i].Number, err)
		}
	}
	allItems = append(allItems, assignedIssues...)

	// Retrieve commented Issues
	commentedIssues, err := fetchIssues(client, ctx, username, "commented", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range commentedIssues {
		commentedIssues[i].Involvement = "commented"
		// Retrieve Issue details (body and comments)
		err = fetchIssueDetails(client, ctx, &commentedIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for Issue (ID: %d): %v\n", commentedIssues[i].Number, err)
		}
	}
	allItems = append(allItems, commentedIssues...)

	// Retrieve created PRs
	createdPRs, err := fetchPRs(client, ctx, username, "created", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range createdPRs {
		createdPRs[i].Involvement = "created"
		// Retrieve PR details (body and comments)
		err = fetchPRDetails(client, ctx, &createdPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for PR (ID: %d): %v\n", createdPRs[i].Number, err)
		}
	}
	allItems = append(allItems, createdPRs...)

	// Retrieve assigned PRs
	assignedPRs, err := fetchPRs(client, ctx, username, "assigned", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range assignedPRs {
		assignedPRs[i].Involvement = "assigned"
		// Retrieve PR details (body and comments)
		err = fetchPRDetails(client, ctx, &assignedPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for PR (ID: %d): %v\n", assignedPRs[i].Number, err)
		}
	}
	allItems = append(allItems, assignedPRs...)

	// Retrieve reviewed PRs
	reviewedPRs, err := fetchPRs(client, ctx, username, "reviewed", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range reviewedPRs {
		reviewedPRs[i].Involvement = "reviewed"
		// Retrieve PR details (body and comments)
		err = fetchPRDetails(client, ctx, &reviewedPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for PR (ID: %d): %v\n", reviewedPRs[i].Number, err)
		}
	}
	allItems = append(allItems, reviewedPRs...)

	return allItems, nil
}

// Retrieve Issues from the GitHub API
func fetchIssues(client *api.RESTClient, ctx context.Context, username, involvement string, dateRange DateRange) ([]Item, error) {
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
	
	items := []Item{}
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
			err = client.Get(pageQuery, &response)
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

			item := Item{
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

// Retrieve PRs from the GitHub API
func fetchPRs(client *api.RESTClient, ctx context.Context, username, involvement string, dateRange DateRange) ([]Item, error) {
	// Query parameters for filtering by date range
	startDateStr := dateRange.StartDate.Format("2006-01-02")
	
	query := fmt.Sprintf("search/issues?q=is:pr+%s:%s+created:>=%s&per_page=100", 
		getInvolvementQuery(involvement), username, startDateStr)
	
	items := []Item{}
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
			err = client.Get(pageQuery, &response)
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

			item := Item{
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

// Return query parameters based on involvement type
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

// Write results to a file
func writeResultsToFile(items []Item, filename, username string, dateRange DateRange, format string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Output based on format
	switch format {
	case "json":
		return writeJSONFormat(file, items)
	case "md":
		return writeMarkdownFormat(file, items, username, dateRange)
	default:
		return fmt.Errorf("Unsupported output format: %s", format)
	}
}

// Output in JSON format
func writeJSONFormat(file *os.File, items []Item) error {
	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	_, err = file.Write(jsonData)
	return err
}

// Output in Markdown format
func writeMarkdownFormat(file *os.File, items []Item, username string, dateRange DateRange) error {
	// Header information
	fmt.Fprintf(file, "# GitHub Activity Report - %s\n", username)
	fmt.Fprintf(file, "Period: %s to %s\n\n", 
		dateRange.StartDate.Format("2006-01-02"), 
		dateRange.EndDate.Format("2006-01-02"))

	// Create summary
	fmt.Fprintf(file, "## Summary\n")
	fmt.Fprintf(file, "- Total items: %d\n", len(items))

	// Count by type
	prCount := 0
	issueCount := 0
	for _, item := range items {
		if item.Type == "PR" {
			prCount++
		} else if item.Type == "Issue" {
			issueCount++
		}
	}
	fmt.Fprintf(file, "- Number of PRs: %d\n", prCount)
	fmt.Fprintf(file, "- Number of Issues: %d\n\n", issueCount)

	// Count by involvement type
	created := 0
	assigned := 0
	commented := 0
	reviewed := 0
	for _, item := range items {
		switch item.Involvement {
		case "created":
			created++
		case "assigned":
			assigned++
		case "commented":
			commented++
		case "reviewed":
			reviewed++
		}
	}
	fmt.Fprintf(file, "- Created items: %d\n", created)
	fmt.Fprintf(file, "- Assigned items: %d\n", assigned)
	fmt.Fprintf(file, "- Commented items: %d\n", commented)
	fmt.Fprintf(file, "- Reviewed items: %d\n\n", reviewed)

	// Detailed list of items
	fmt.Fprintf(file, "## Item Details\n\n")
	
	// First, created items
	if created > 0 {
		fmt.Fprintf(file, "### Created Items\n\n")
		for _, item := range items {
			if item.Involvement == "created" {
				writeItemDetails(file, item)
			}
		}
	}
	
	// Assigned items
	if assigned > 0 {
		fmt.Fprintf(file, "### Assigned Items\n\n")
		for _, item := range items {
			if item.Involvement == "assigned" {
				writeItemDetails(file, item)
			}
		}
	}
	
	// Commented items
	if commented > 0 {
		fmt.Fprintf(file, "### Commented Items\n\n")
		for _, item := range items {
			if item.Involvement == "commented" {
				writeItemDetails(file, item)
			}
		}
	}
	
	// Reviewed items
	if reviewed > 0 {
		fmt.Fprintf(file, "### Reviewed Items\n\n")
		for _, item := range items {
			if item.Involvement == "reviewed" {
				writeItemDetails(file, item)
			}
		}
	}

	return nil
}

// Write item details to the file
func writeItemDetails(file *os.File, item Item) {
	fmt.Fprintf(file, "- [%s #%d] %s\n", item.Type, item.Number, item.Title)
	fmt.Fprintf(file, "  - URL: %s\n", item.URL)
	fmt.Fprintf(file, "  - Repository: %s\n", item.Repository)
	fmt.Fprintf(file, "  - State: %s\n", item.State)
	fmt.Fprintf(file, "  - Created on: %s\n", item.CreatedAt.Format("2006-01-02"))
	fmt.Fprintf(file, "  - Updated on: %s\n", item.UpdatedAt.Format("2006-01-02"))
	
	if len(item.Assignees) > 0 {
		fmt.Fprintf(file, "  - Assignees: %s\n", strings.Join(item.Assignees, ", "))
	}
	
	if len(item.Labels) > 0 {
		fmt.Fprintf(file, "  - Labels: %s\n", strings.Join(item.Labels, ", "))
	}

	// Output the body
	if item.Body != "" {
		// If the body is long, truncate it appropriately
		body := item.Body
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		fmt.Fprintf(file, "  - Body:\n    %s\n", strings.ReplaceAll(body, "\n", "\n    "))
	}
	
	// Output comments
	if len(item.Comments) > 0 {
		fmt.Fprintf(file, "  - Comments (%d):\n", len(item.Comments))
		
		// Limit the number of comments displayed
		maxComments := 5
		if len(item.Comments) > maxComments {
			fmt.Fprintf(file, "    (Only the first %d shown)\n", maxComments)
		}
		
		count := 0
		for _, comment := range item.Comments {
			if count >= maxComments {
				break
			}
			
			// If the comment body is long, truncate it appropriately
			body := comment.Body
			if len(body) > 200 {
				body = body[:200] + "..."
			}
			
			fmt.Fprintf(file, "    - %s (%s):\n      %s\n", 
				comment.Author, 
				comment.CreatedAt.Format("2006-01-02"),
				strings.ReplaceAll(body, "\n", "\n      "))
			
			count++
		}
	}
	
	fmt.Fprintln(file, "")
}

// Retrieve details (body and comments) of an Issue
func fetchIssueDetails(client *api.RESTClient, ctx context.Context, item *Item) error {
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
		err = client.Get(issueURL, &issueDetail)
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
	return fetchComments(client, ctx, item, fmt.Sprintf("repos/%s/issues/%d/comments", repoPath, item.Number))
}

// Retrieve details (body and comments) of a PR
func fetchPRDetails(client *api.RESTClient, ctx context.Context, item *Item) error {
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
		err = client.Get(prURL, &prDetail)
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
	err = fetchComments(client, ctx, item, issueCommentsURL)
	if err != nil {
		return err
	}
	
	// Also retrieve PR review comments
	reviewCommentsURL := fmt.Sprintf("repos/%s/pulls/%d/comments", repoPath, item.Number)
	return fetchReviewComments(client, ctx, item, reviewCommentsURL)
}

// Common function to retrieve comments
func fetchComments(client *api.RESTClient, ctx context.Context, item *Item, commentsURL string) error {
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
		err = client.Get(commentsURL, &comments)
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
		item.Comments = append(item.Comments, Comment{
			Author:    c.User.Login,
			Body:      c.Body,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	
	return nil
}

// Function to retrieve PR review comments
func fetchReviewComments(client *api.RESTClient, ctx context.Context, item *Item, reviewCommentsURL string) error {
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
		err = client.Get(reviewCommentsURL, &reviewComments)
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
		item.Comments = append(item.Comments, Comment{
			Author:    rc.User.Login,
			Body:      rc.Body,
			CreatedAt: rc.CreatedAt,
			UpdatedAt: rc.UpdatedAt,
		})
	}
	
	return nil
}

// Function to extract the path from a repository URL
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

// Function to filter out comments from specific users
func filterIgnoredUserComments(items []Item, ignoreUsers []string) {
	for i := range items {
		var filteredComments []Comment
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

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
