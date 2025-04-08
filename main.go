package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"git.pepabo.com/yukyan/gh-pric/github"
	"git.pepabo.com/yukyan/gh-pric/github/output"
	"git.pepabo.com/yukyan/gh-pric/github/util"
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
	dateRange, err := util.ParseDateRange(startDateStr, endDateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse dates: %v\n", err)
		os.Exit(1)
	}

	// Initialize GitHub client
	client, err := github.NewClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize GitHub client: %v\n", err)
		os.Exit(1)
	}

	// Retrieve user information
	username, err := client.GetUsername()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to retrieve user information: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Retrieving GitHub activity for user '%s'...\n", username)
	fmt.Printf("Period: %s to %s\n", dateRange.StartDate.Format("2006-01-02"), dateRange.EndDate.Format("2006-01-02"))

	// Data retrieval
	items, err := client.FetchAllItems(username, dateRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to retrieve data: %v\n", err)
		os.Exit(1)
	}

	// Filter comments from specific users
	if len(ignoreUsers) > 0 {
		github.FilterIgnoredUserComments(items, ignoreUsers)
	}

	// Output results
	err = output.WriteResults(items, outputFile, username, dateRange, outputFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Results saved to %s\n", outputFile)
}

// For more examples of using go-gh, see:
// https://github.com/cli/go-gh/blob/trunk/example_gh_test.go
