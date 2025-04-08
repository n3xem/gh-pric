# gh-pric

A GitHub CLI extension that outputs a summary of your PRs and Issues created within a specified period into a single text file.

## Features

- Retrieves GitHub activities (PR/Issues) within a specified period
- Can filter by the following involvement types:
  - Items you created
  - Items assigned to you
  - Items you commented on
  - Items you reviewed (PRs only)
- Outputs results to a text file (Markdown or JSON format)
- Respects GitHub API rate limits
- Can retrieve comment details

## Installation

After ensuring GitHub CLI (`gh`) is installed:

```bash
gh extension install n3xem/gh-pric
```

## Usage

Basic usage:

```bash
gh pric
```

Run with a specified period:

```bash
gh pric --from 2023-01-01 --to 2023-12-31
```

Specify output filename:

```bash
gh pric --output my-github-activity.txt
```

Specify JSON output format:

```bash
gh pric --output-format json
```

Exclude comments from specific users:

```bash
gh pric --comment-ignore user1,user2
```

Using all options:

```bash
gh pric --from 2023-01-01 --to 2023-12-31 --output my-github-activity.txt --output-format md --comment-ignore bot1,bot2
```

## Options

| Option | Default Value | Description |
|--------|---------------|-------------|
| `--from` | 3 days ago | Start date (YYYY-MM-DD format) |
| `--to` | today | End date (YYYY-MM-DD format) |
| `--output`, `-o` | github-activity.txt | Output filename |
| `--output-format` | md | Output format (md or json) |
| `--comment-ignore` | none | Usernames whose comments to exclude (comma-separated) |

## Output Example

The generated file will have the following structure:

```
# GitHub Activity Report - username
Period: 2023-01-01 to 2023-12-31

## Summary
- Total items: 42
- Number of PRs: 25
- Number of Issues: 17
- Created items: 15
- Assigned items: 10
- Commented items: 12
- Reviewed items: 5

## Item Details

### Created Items
- [PR #123] Title
  - URL: https://github.com/org/repo/pull/123
  - Repository: org/repo
  - Status: merged
  - Created: 2023-03-15
  - Updated: 2023-03-20
  - Assignees: user1, user2
  - Labels: bug, enhancement
  - Body: PR content (truncated if too long)
  - Comments (3):
    - username (2023-03-16): Comment content (truncated if too long)
    - ...

...(continued)
```

## Notes

- You may hit GitHub API rate limits if you have many repositories or activities
- There are limits to how much data can be retrieved at once (maximum 10 pages)
- Proper permissions are required to fetch private repository information
- Only the first 5 comments are shown when there are many comments
- Long body text and comments are automatically truncated

## License

MIT 
