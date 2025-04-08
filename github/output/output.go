package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"git.pepabo.com/yukyan/gh-pric/github/model"
)

// WriteResults は結果をファイルに出力します
func WriteResults(items []model.Item, filename, username string, dateRange model.DateRange, format string) error {
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

// JSON形式で出力
func writeJSONFormat(file *os.File, items []model.Item) error {
	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	_, err = file.Write(jsonData)
	return err
}

// Markdown形式で出力
func writeMarkdownFormat(file *os.File, items []model.Item, username string, dateRange model.DateRange) error {
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

// アイテムの詳細をファイルに書き出す
func writeItemDetails(file *os.File, item model.Item) {
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
