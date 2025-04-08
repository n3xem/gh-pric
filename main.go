package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"git.pepabo.com/yukyan/gh-pric/github"
	"git.pepabo.com/yukyan/gh-pric/github/model"
	"git.pepabo.com/yukyan/gh-pric/github/output"
	"git.pepabo.com/yukyan/gh-pric/github/util"
	"github.com/briandowns/spinner"
)

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
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = " Parsing date range..."
	s.Start()
	dateRange, err := util.ParseDateRange(startDateStr, endDateStr)
	s.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse dates: %v\n", err)
		os.Exit(1)
	}

	// Initialize GitHub client
	s.Suffix = " Initializing GitHub client..."
	s.Start()
	client, err := github.NewClient()
	s.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize GitHub client: %v\n", err)
		os.Exit(1)
	}

	// Retrieve user information
	s.Suffix = " Retrieving user information..."
	s.Start()
	username, err := client.GetUsername()
	s.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to retrieve user information: %v\n", err)
		os.Exit(1)
	}

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
		s.Suffix = " Filtering comments from ignored users..."
		s.Start()
		github.FilterIgnoredUserComments(items, ignoreUsers)
		s.Stop()
	}

	// Output results
	s.Suffix = " Writing results to file..."
	s.Start()
	err = output.WriteResults(items, outputFile, username, dateRange, outputFormat)
	s.Stop()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Results saved to %s\n", outputFile)
}

// fetchAllItems は指定したユーザーの全ての項目（PR、Issue）を取得します
func fetchAllItems(client *github.Client, username string, dateRange model.DateRange) ([]model.Item, error) {
	var allItems []model.Item
	ctx := context.Background()
	
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)

	// Retrieve created Issues
	s.Suffix = " 作成したIssueを取得中..."
	s.Start()
	createdIssues, err := client.FetchIssues(ctx, username, "created", dateRange)
	s.Stop()
	if err != nil {
		return nil, err
	}
	
	s.Suffix = " 作成したIssueの詳細を取得中..."
	s.Start()
	for i := range createdIssues {
		createdIssues[i].Involvement = "created"
		// Retrieve Issue details (body and comments)
		err = client.FetchIssueDetails(ctx, &createdIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for Issue (ID: %d): %v\n", createdIssues[i].Number, err)
		}
	}
	s.Stop()
	allItems = append(allItems, createdIssues...)

	// Retrieve assigned Issues
	s.Suffix = " アサインされたIssueを取得中..."
	s.Start()
	assignedIssues, err := client.FetchIssues(ctx, username, "assigned", dateRange)
	s.Stop()
	if err != nil {
		return nil, err
	}
	
	s.Suffix = " アサインされたIssueの詳細を取得中..."
	s.Start()
	for i := range assignedIssues {
		assignedIssues[i].Involvement = "assigned"
		// Retrieve Issue details (body and comments)
		err = client.FetchIssueDetails(ctx, &assignedIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for Issue (ID: %d): %v\n", assignedIssues[i].Number, err)
		}
	}
	s.Stop()
	allItems = append(allItems, assignedIssues...)

	// Retrieve commented Issues
	s.Suffix = " コメントしたIssueを取得中..."
	s.Start()
	commentedIssues, err := client.FetchIssues(ctx, username, "commented", dateRange)
	s.Stop()
	if err != nil {
		return nil, err
	}
	
	s.Suffix = " コメントしたIssueの詳細を取得中..."
	s.Start()
	for i := range commentedIssues {
		commentedIssues[i].Involvement = "commented"
		// Retrieve Issue details (body and comments)
		err = client.FetchIssueDetails(ctx, &commentedIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for Issue (ID: %d): %v\n", commentedIssues[i].Number, err)
		}
	}
	s.Stop()
	allItems = append(allItems, commentedIssues...)

	// Retrieve created PRs
	s.Suffix = " 作成したPRを取得中..."
	s.Start()
	createdPRs, err := client.FetchPRs(ctx, username, "created", dateRange)
	s.Stop()
	if err != nil {
		return nil, err
	}
	
	s.Suffix = " 作成したPRの詳細を取得中..."
	s.Start()
	for i := range createdPRs {
		createdPRs[i].Involvement = "created"
		// Retrieve PR details (body and comments)
		err = client.FetchPRDetails(ctx, &createdPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for PR (ID: %d): %v\n", createdPRs[i].Number, err)
		}
	}
	s.Stop()
	allItems = append(allItems, createdPRs...)

	// Retrieve assigned PRs
	s.Suffix = " アサインされたPRを取得中..."
	s.Start()
	assignedPRs, err := client.FetchPRs(ctx, username, "assigned", dateRange)
	s.Stop()
	if err != nil {
		return nil, err
	}
	
	s.Suffix = " アサインされたPRの詳細を取得中..."
	s.Start()
	for i := range assignedPRs {
		assignedPRs[i].Involvement = "assigned"
		// Retrieve PR details (body and comments)
		err = client.FetchPRDetails(ctx, &assignedPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for PR (ID: %d): %v\n", assignedPRs[i].Number, err)
		}
	}
	s.Stop()
	allItems = append(allItems, assignedPRs...)

	// Retrieve reviewed PRs
	s.Suffix = " レビューしたPRを取得中..."
	s.Start()
	reviewedPRs, err := client.FetchPRs(ctx, username, "reviewed", dateRange)
	s.Stop()
	if err != nil {
		return nil, err
	}
	
	s.Suffix = " レビューしたPRの詳細を取得中..."
	s.Start()
	for i := range reviewedPRs {
		reviewedPRs[i].Involvement = "reviewed"
		// Retrieve PR details (body and comments)
		err = client.FetchPRDetails(ctx, &reviewedPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to retrieve details for PR (ID: %d): %v\n", reviewedPRs[i].Number, err)
		}
	}
	s.Stop()
	allItems = append(allItems, reviewedPRs...)

	return allItems, nil
}
