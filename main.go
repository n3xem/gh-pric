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

// 期間の設定用構造体
type DateRange struct {
	StartDate time.Time
	EndDate   time.Time
}

// PRとIssueの情報を格納する構造体
type Item struct {
	Type        string    // "PR" または "Issue"
	Number      int       // PR番号またはIssue番号
	Title       string    // タイトル
	URL         string    // URL
	State       string    // 状態（open, closed, merged）
	CreatedAt   time.Time // 作成日時
	UpdatedAt   time.Time // 更新日時
	Author      string    // 作成者
	Assignees   []string  // アサイン先
	Labels      []string  // ラベル
	Repository  string    // リポジトリ名
	Involvement string    // 関与タイプ（created, assigned, commented）
	Body        string    // 本文
	Comments    []Comment // コメント
}

// コメント情報を格納する構造体
type Comment struct {
	Author    string    // コメント投稿者
	Body      string    // コメント本文
	CreatedAt time.Time // 投稿日時
	UpdatedAt time.Time // 更新日時
}

func main() {
	// コマンドライン引数の解析
	var startDateStr, endDateStr, outputFile string
	var commentIgnoreUsers string
	var outputFormat string
	var defaultEndDate = time.Now().Format("2006-01-02")
	var defaultStartDate = time.Now().AddDate(0, 0, -3).Format("2006-01-02") // デフォルトで3日前

	flag.StringVar(&startDateStr, "from", defaultStartDate, "開始日 (YYYY-MM-DD形式)")
	flag.StringVar(&endDateStr, "to", defaultEndDate, "終了日 (YYYY-MM-DD形式)")
	flag.StringVar(&outputFile, "output", "github-activity.txt", "出力ファイル名")
	flag.StringVar(&outputFile, "o", "github-activity.txt", "出力ファイル名 (--outputのエイリアス)")
	flag.StringVar(&commentIgnoreUsers, "comment-ignore", "", "出力に含めないコメントのユーザー名（カンマ区切りで複数指定可能）")
	flag.StringVar(&outputFormat, "output-format", "md", "出力フォーマット (md または json)")
	flag.Parse()

	// 出力フォーマットのバリデーション
	if outputFormat != "md" && outputFormat != "json" {
		fmt.Fprintf(os.Stderr, "無効な出力フォーマットです: %s (md または json を指定してください)\n", outputFormat)
		os.Exit(1)
	}

	// コメント除外ユーザーのリストを作成
	var ignoreUsers []string
	if commentIgnoreUsers != "" {
		ignoreUsers = strings.Split(commentIgnoreUsers, ",")
		for i, user := range ignoreUsers {
			ignoreUsers[i] = strings.TrimSpace(user)
		}
	}

	// 日付のパース
	dateRange, err := parseDateRange(startDateStr, endDateStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "日付の解析に失敗しました: %v\n", err)
		os.Exit(1)
	}

	// GitHubクライアントの初期化
	client, err := api.DefaultRESTClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHubクライアントの初期化に失敗しました: %v\n", err)
		os.Exit(1)
	}

	// ユーザー情報の取得
	userInfo := struct {
		Login string `json:"login"`
	}{}
	err = client.Get("user", &userInfo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ユーザー情報の取得に失敗しました: %v\n", err)
		os.Exit(1)
	}

	username := userInfo.Login
	fmt.Printf("ユーザー '%s' のGitHub活動を取得しています...\n", username)
	fmt.Printf("期間: %s から %s まで\n", dateRange.StartDate.Format("2006-01-02"), dateRange.EndDate.Format("2006-01-02"))

	// データ取得
	items, err := fetchAllItems(client, username, dateRange)
	if err != nil {
		fmt.Fprintf(os.Stderr, "データの取得に失敗しました: %v\n", err)
		os.Exit(1)
	}

	// 特定ユーザーのコメントをフィルタリング
	if len(ignoreUsers) > 0 {
		filterIgnoredUserComments(items, ignoreUsers)
	}

	// 結果の出力
	err = writeResultsToFile(items, outputFile, username, dateRange, outputFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ファイルへの書き込みに失敗しました: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("結果を %s に保存しました\n", outputFile)
}

// 日付文字列をパースして期間を返す
func parseDateRange(startStr, endStr string) (DateRange, error) {
	startDate, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return DateRange{}, fmt.Errorf("開始日の解析に失敗しました: %w", err)
	}

	endDate, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return DateRange{}, fmt.Errorf("終了日の解析に失敗しました: %w", err)
	}

	// 終了日は23:59:59に設定
	endDate = endDate.Add(24*time.Hour - time.Second)

	if endDate.Before(startDate) {
		return DateRange{}, fmt.Errorf("終了日は開始日より後である必要があります")
	}

	return DateRange{
		StartDate: startDate,
		EndDate:   endDate,
	}, nil
}

// GitHub APIからユーザーに関連するPRとIssueを取得
func fetchAllItems(client *api.RESTClient, username string, dateRange DateRange) ([]Item, error) {
	var allItems []Item
	ctx := context.Background()

	// 作成したIssueの取得
	createdIssues, err := fetchIssues(client, ctx, username, "created", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range createdIssues {
		createdIssues[i].Involvement = "created"
		// Issue詳細情報の取得（本文とコメント）
		err = fetchIssueDetails(client, ctx, &createdIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Issueの詳細取得に失敗しました（ID: %d）: %v\n", createdIssues[i].Number, err)
		}
	}
	allItems = append(allItems, createdIssues...)

	// アサインされたIssueの取得
	assignedIssues, err := fetchIssues(client, ctx, username, "assigned", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range assignedIssues {
		assignedIssues[i].Involvement = "assigned"
		// Issue詳細情報の取得（本文とコメント）
		err = fetchIssueDetails(client, ctx, &assignedIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Issueの詳細取得に失敗しました（ID: %d）: %v\n", assignedIssues[i].Number, err)
		}
	}
	allItems = append(allItems, assignedIssues...)

	// コメントしたIssueの取得
	commentedIssues, err := fetchIssues(client, ctx, username, "commented", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range commentedIssues {
		commentedIssues[i].Involvement = "commented"
		// Issue詳細情報の取得（本文とコメント）
		err = fetchIssueDetails(client, ctx, &commentedIssues[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Issueの詳細取得に失敗しました（ID: %d）: %v\n", commentedIssues[i].Number, err)
		}
	}
	allItems = append(allItems, commentedIssues...)

	// 作成したPRの取得
	createdPRs, err := fetchPRs(client, ctx, username, "created", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range createdPRs {
		createdPRs[i].Involvement = "created"
		// PR詳細情報の取得（本文とコメント）
		err = fetchPRDetails(client, ctx, &createdPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "PRの詳細取得に失敗しました（ID: %d）: %v\n", createdPRs[i].Number, err)
		}
	}
	allItems = append(allItems, createdPRs...)

	// アサインされたPRの取得
	assignedPRs, err := fetchPRs(client, ctx, username, "assigned", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range assignedPRs {
		assignedPRs[i].Involvement = "assigned"
		// PR詳細情報の取得（本文とコメント）
		err = fetchPRDetails(client, ctx, &assignedPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "PRの詳細取得に失敗しました（ID: %d）: %v\n", assignedPRs[i].Number, err)
		}
	}
	allItems = append(allItems, assignedPRs...)

	// レビューしたPRの取得
	reviewedPRs, err := fetchPRs(client, ctx, username, "reviewed", dateRange)
	if err != nil {
		return nil, err
	}
	for i := range reviewedPRs {
		reviewedPRs[i].Involvement = "reviewed"
		// PR詳細情報の取得（本文とコメント）
		err = fetchPRDetails(client, ctx, &reviewedPRs[i])
		if err != nil {
			fmt.Fprintf(os.Stderr, "PRの詳細取得に失敗しました（ID: %d）: %v\n", reviewedPRs[i].Number, err)
		}
	}
	allItems = append(allItems, reviewedPRs...)

	return allItems, nil
}

// GitHub APIからIssueを取得
func fetchIssues(client *api.RESTClient, ctx context.Context, username, involvement string, dateRange DateRange) ([]Item, error) {
	// 日付範囲でフィルタリングするためのクエリパラメータ
	startDateStr := dateRange.StartDate.Format("2006-01-02")
	
	// 関連ごとに適切なクエリパラメータを構築
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
		
		// リトライ機能を追加
		var err error
		maxRetries := 3
		for retryCount := 0; retryCount < maxRetries; retryCount++ {
			err = client.Get(pageQuery, &response)
			if err == nil {
				break
			}
			
			// リトライ前に待機
			time.Sleep(2 * time.Second)
		}
		
		if err != nil {
			return nil, fmt.Errorf("Issueの取得に失敗しました: %w", err)
		}
		
		// レスポンスが空の場合は終了
		if len(response.Items) == 0 {
			hasMore = false
			continue
		}

		for _, issue := range response.Items {
			// 日付範囲外のものはスキップ
			if issue.CreatedAt.After(dateRange.EndDate) || issue.CreatedAt.Before(dateRange.StartDate) {
				continue
			}

			// リポジトリ名の抽出
			repoURL := issue.RepositoryURL
			repoParts := strings.Split(repoURL, "/")
			repoName := ""
			if len(repoParts) >= 2 {
				repoName = fmt.Sprintf("%s/%s", repoParts[len(repoParts)-2], repoParts[len(repoParts)-1])
			}

			// アサイン先の抽出
			assignees := make([]string, len(issue.Assignees))
			for i, a := range issue.Assignees {
				assignees[i] = a.Login
			}

			// ラベルの抽出
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

		// Rate Limitに配慮
		time.Sleep(1 * time.Second)
		page++
		
		// 一定数以上取得したら終了（オプション）
		if page > 10 {
			hasMore = false
		}
	}

	return items, nil
}

// GitHub APIからPRを取得
func fetchPRs(client *api.RESTClient, ctx context.Context, username, involvement string, dateRange DateRange) ([]Item, error) {
	// 日付範囲でフィルタリングするためのクエリパラメータ
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
		
		// リトライ機能を追加
		var err error
		maxRetries := 3
		for retryCount := 0; retryCount < maxRetries; retryCount++ {
			err = client.Get(pageQuery, &response)
			if err == nil {
				break
			}
			
			// リトライ前に待機
			time.Sleep(2 * time.Second)
		}
		
		if err != nil {
			return nil, fmt.Errorf("PRの取得に失敗しました: %w", err)
		}
		
		// レスポンスが空の場合は終了
		if len(response.Items) == 0 {
			hasMore = false
			continue
		}

		for _, pr := range response.Items {
			// 日付範囲外のものはスキップ
			if pr.CreatedAt.After(dateRange.EndDate) || pr.CreatedAt.Before(dateRange.StartDate) {
				continue
			}

			// リポジトリ名の抽出
			repoURL := pr.RepositoryURL
			repoParts := strings.Split(repoURL, "/")
			repoName := ""
			if len(repoParts) >= 2 {
				repoName = fmt.Sprintf("%s/%s", repoParts[len(repoParts)-2], repoParts[len(repoParts)-1])
			}

			// アサイン先の抽出
			assignees := make([]string, len(pr.Assignees))
			for i, a := range pr.Assignees {
				assignees[i] = a.Login
			}

			// ラベルの抽出
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

		// Rate Limitに配慮
		time.Sleep(1 * time.Second)
		page++
		
		// 一定数以上取得したら終了（オプション）
		if page > 10 {
			hasMore = false
		}
	}

	return items, nil
}

// 関与タイプに応じたクエリパラメータを返す
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

// 結果をファイルに書き込む
func writeResultsToFile(items []Item, filename, username string, dateRange DateRange, format string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// フォーマットに応じて出力
	switch format {
	case "json":
		return writeJSONFormat(file, items)
	case "md":
		return writeMarkdownFormat(file, items, username, dateRange)
	default:
		return fmt.Errorf("未対応の出力フォーマット: %s", format)
	}
}

// JSON形式で出力
func writeJSONFormat(file *os.File, items []Item) error {
	jsonData, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	_, err = file.Write(jsonData)
	return err
}

// マークダウン形式で出力
func writeMarkdownFormat(file *os.File, items []Item, username string, dateRange DateRange) error {
	// ヘッダー情報
	fmt.Fprintf(file, "# GitHub活動レポート - %s\n", username)
	fmt.Fprintf(file, "期間: %s から %s まで\n\n", 
		dateRange.StartDate.Format("2006-01-02"), 
		dateRange.EndDate.Format("2006-01-02"))

	// サマリーを作成
	fmt.Fprintf(file, "## サマリー\n")
	fmt.Fprintf(file, "- 合計アイテム数: %d\n", len(items))

	// タイプ別カウント
	prCount := 0
	issueCount := 0
	for _, item := range items {
		if item.Type == "PR" {
			prCount++
		} else if item.Type == "Issue" {
			issueCount++
		}
	}
	fmt.Fprintf(file, "- PRの数: %d\n", prCount)
	fmt.Fprintf(file, "- Issueの数: %d\n\n", issueCount)

	// 関与タイプ別カウント
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
	fmt.Fprintf(file, "- 作成したアイテム: %d\n", created)
	fmt.Fprintf(file, "- アサインされたアイテム: %d\n", assigned)
	fmt.Fprintf(file, "- コメントしたアイテム: %d\n", commented)
	fmt.Fprintf(file, "- レビューしたアイテム: %d\n\n", reviewed)

	// アイテムの詳細リスト
	fmt.Fprintf(file, "## アイテム詳細\n\n")
	
	// まず作成したもの
	if created > 0 {
		fmt.Fprintf(file, "### 作成したアイテム\n\n")
		for _, item := range items {
			if item.Involvement == "created" {
				writeItemDetails(file, item)
			}
		}
	}
	
	// アサインされたもの
	if assigned > 0 {
		fmt.Fprintf(file, "### アサインされたアイテム\n\n")
		for _, item := range items {
			if item.Involvement == "assigned" {
				writeItemDetails(file, item)
			}
		}
	}
	
	// コメントしたもの
	if commented > 0 {
		fmt.Fprintf(file, "### コメントしたアイテム\n\n")
		for _, item := range items {
			if item.Involvement == "commented" {
				writeItemDetails(file, item)
			}
		}
	}
	
	// レビューしたもの
	if reviewed > 0 {
		fmt.Fprintf(file, "### レビューしたアイテム\n\n")
		for _, item := range items {
			if item.Involvement == "reviewed" {
				writeItemDetails(file, item)
			}
		}
	}

	return nil
}

// アイテムの詳細を書き込む
func writeItemDetails(file *os.File, item Item) {
	fmt.Fprintf(file, "- [%s #%d] %s\n", item.Type, item.Number, item.Title)
	fmt.Fprintf(file, "  - URL: %s\n", item.URL)
	fmt.Fprintf(file, "  - リポジトリ: %s\n", item.Repository)
	fmt.Fprintf(file, "  - 状態: %s\n", item.State)
	fmt.Fprintf(file, "  - 作成日: %s\n", item.CreatedAt.Format("2006-01-02"))
	fmt.Fprintf(file, "  - 更新日: %s\n", item.UpdatedAt.Format("2006-01-02"))
	
	if len(item.Assignees) > 0 {
		fmt.Fprintf(file, "  - アサイン先: %s\n", strings.Join(item.Assignees, ", "))
	}
	
	if len(item.Labels) > 0 {
		fmt.Fprintf(file, "  - ラベル: %s\n", strings.Join(item.Labels, ", "))
	}

	// 本文も出力
	if item.Body != "" {
		// 本文が長い場合は適切に省略
		body := item.Body
		if len(body) > 300 {
			body = body[:300] + "..."
		}
		fmt.Fprintf(file, "  - 本文:\n    %s\n", strings.ReplaceAll(body, "\n", "\n    "))
	}
	
	// コメントの出力
	if len(item.Comments) > 0 {
		fmt.Fprintf(file, "  - コメント (%d件):\n", len(item.Comments))
		
		// コメント数が多い場合は制限
		maxComments := 5
		if len(item.Comments) > maxComments {
			fmt.Fprintf(file, "    (最初の%d件のみ表示)\n", maxComments)
		}
		
		count := 0
		for _, comment := range item.Comments {
			if count >= maxComments {
				break
			}
			
			// コメント本文が長い場合は適切に省略
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

// Issueの詳細（本文とコメント）を取得する
func fetchIssueDetails(client *api.RESTClient, ctx context.Context, item *Item) error {
	// リポジトリ名とIssue番号を抽出
	repoPath := getRepoPathFromURL(item.Repository)
	if repoPath == "" {
		return fmt.Errorf("リポジトリパスの抽出に失敗しました: %s", item.Repository)
	}
	
	// Issueの詳細情報を取得
	var issueDetail struct {
		Body string `json:"body"`
	}
	
	issueURL := fmt.Sprintf("repos/%s/issues/%d", repoPath, item.Number)
	
	// リトライ機能を使用
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = client.Get(issueURL, &issueDetail)
		if err == nil {
			break
		}
		
		// リトライ前に待機
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("Issueの詳細取得に失敗しました: %w", err)
	}
	
	item.Body = issueDetail.Body
	
	// コメントを取得
	return fetchComments(client, ctx, item, fmt.Sprintf("repos/%s/issues/%d/comments", repoPath, item.Number))
}

// PRの詳細（本文とコメント）を取得する
func fetchPRDetails(client *api.RESTClient, ctx context.Context, item *Item) error {
	// リポジトリ名とPR番号を抽出
	repoPath := getRepoPathFromURL(item.Repository)
	if repoPath == "" {
		return fmt.Errorf("リポジトリパスの抽出に失敗しました: %s", item.Repository)
	}
	
	// PRの詳細情報を取得（PRはIssueのエンドポイントでも取得可能）
	var prDetail struct {
		Body string `json:"body"`
	}
	
	prURL := fmt.Sprintf("repos/%s/pulls/%d", repoPath, item.Number)
	
	// リトライ機能を使用
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = client.Get(prURL, &prDetail)
		if err == nil {
			break
		}
		
		// リトライ前に待機
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("PRの詳細取得に失敗しました: %w", err)
	}
	
	item.Body = prDetail.Body
	
	// コメントを取得
	issueCommentsURL := fmt.Sprintf("repos/%s/issues/%d/comments", repoPath, item.Number)
	err = fetchComments(client, ctx, item, issueCommentsURL)
	if err != nil {
		return err
	}
	
	// PRレビューコメントも取得
	reviewCommentsURL := fmt.Sprintf("repos/%s/pulls/%d/comments", repoPath, item.Number)
	return fetchReviewComments(client, ctx, item, reviewCommentsURL)
}

// コメントを取得する共通関数
func fetchComments(client *api.RESTClient, ctx context.Context, item *Item, commentsURL string) error {
	var comments []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	
	// リトライ機能を使用
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = client.Get(commentsURL, &comments)
		if err == nil {
			break
		}
		
		// リトライ前に待機
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("コメントの取得に失敗しました: %w", err)
	}
	
	// コメントをItem構造体に追加
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

// PRレビューコメントを取得する関数
func fetchReviewComments(client *api.RESTClient, ctx context.Context, item *Item, reviewCommentsURL string) error {
	var reviewComments []struct {
		User struct {
			Login string `json:"login"`
		} `json:"user"`
		Body      string    `json:"body"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}
	
	// リトライ機能を使用
	var err error
	maxRetries := 3
	for retryCount := 0; retryCount < maxRetries; retryCount++ {
		err = client.Get(reviewCommentsURL, &reviewComments)
		if err == nil {
			break
		}
		
		// リトライ前に待機
		time.Sleep(2 * time.Second)
	}
	
	if err != nil {
		return fmt.Errorf("レビューコメントの取得に失敗しました: %w", err)
	}
	
	// レビューコメントをItem構造体に追加
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

// リポジトリURLからパスを抽出する関数
func getRepoPathFromURL(repoURL string) string {
	// まずリポジトリURL形式を確認
	if strings.HasPrefix(repoURL, "http") {
		// URLからパスを抽出（例: https://github.com/owner/repo → owner/repo）
		u, err := url.Parse(repoURL)
		if err != nil {
			return ""
		}
		path := strings.TrimPrefix(u.Path, "/")
		return path
	} else if strings.Contains(repoURL, "/") {
		// すでにowner/repo形式の場合はそのまま返す
		return repoURL
	}
	
	return ""
}

// 特定ユーザーのコメントを除外する関数
func filterIgnoredUserComments(items []Item, ignoreUsers []string) {
	for i := range items {
		var filteredComments []Comment
		for _, comment := range items[i].Comments {
			// ユーザーがignoreUsersリストにいなければコメントを残す
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
