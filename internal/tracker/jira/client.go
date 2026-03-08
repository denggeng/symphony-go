package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/tracker"
)

var ErrUnsupportedTrackerKind = errors.New("unsupported tracker kind")

type Client struct {
	cfg        config.Config
	logger     *slog.Logger
	httpClient *http.Client
}

func New(cfg config.Config, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		cfg:    cfg,
		logger: logger,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (client *Client) Kind() string {
	return "jira"
}

func (client *Client) FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error) {
	jql := strings.TrimSpace(client.cfg.Tracker.JQL)
	if jql == "" {
		jql = client.defaultCandidateJQL()
	}

	return client.searchIssues(ctx, jql)
}

func (client *Client) FetchIssuesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error) {
	issues := make([]domain.Issue, 0, len(ids))
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			continue
		}
		issue, err := client.fetchIssue(ctx, id)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, nil
}

func (client *Client) FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error) {
	if len(states) == 0 {
		return []domain.Issue{}, nil
	}
	jql := client.jqlForStates(states)
	return client.searchIssues(ctx, jql)
}

func (client *Client) CreateComment(ctx context.Context, issueIdentifier string, body string) error {
	request := tracker.RawRequest{
		Method: http.MethodPost,
		Path:   "/rest/api/3/issue/" + issueIdentifier + "/comment",
		Body: map[string]any{
			"body": plainTextADF(body),
		},
	}

	response, err := client.RawAPI(ctx, request)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("jira create comment failed with status %d", response.StatusCode)
	}
	return nil
}

func (client *Client) UpdateIssueState(ctx context.Context, issueIdentifier string, stateName string) error {
	response, err := client.RawAPI(ctx, tracker.RawRequest{
		Method: http.MethodGet,
		Path:   "/rest/api/3/issue/" + issueIdentifier + "/transitions",
	})
	if err != nil {
		return err
	}
	payload, ok := response.Body.(map[string]any)
	if !ok {
		return fmt.Errorf("jira transitions response was not an object")
	}
	transitions, _ := payload["transitions"].([]any)
	transitionID := ""
	for _, raw := range transitions {
		transition, _ := raw.(map[string]any)
		toState, _ := transition["to"].(map[string]any)
		name, _ := toState["name"].(string)
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(stateName)) {
			transitionID, _ = transition["id"].(string)
			break
		}
	}
	if transitionID == "" {
		return fmt.Errorf("jira transition not found for state %q", stateName)
	}
	result, err := client.RawAPI(ctx, tracker.RawRequest{
		Method: http.MethodPost,
		Path:   "/rest/api/3/issue/" + issueIdentifier + "/transitions",
		Body: map[string]any{
			"transition": map[string]any{"id": transitionID},
		},
	})
	if err != nil {
		return err
	}
	if result.StatusCode < 200 || result.StatusCode >= 300 {
		return fmt.Errorf("jira transition failed with status %d", result.StatusCode)
	}
	return nil
}

func (client *Client) RawAPI(ctx context.Context, request tracker.RawRequest) (tracker.RawResponse, error) {
	if strings.TrimSpace(client.cfg.Tracker.BaseURL) == "" {
		return tracker.RawResponse{}, errors.New("tracker.base_url is required for Jira")
	}

	method := strings.ToUpper(strings.TrimSpace(request.Method))
	if method == "" {
		method = http.MethodGet
	}

	endpoint, err := url.Parse(strings.TrimRight(client.cfg.Tracker.BaseURL, "/"))
	if err != nil {
		return tracker.RawResponse{}, fmt.Errorf("parse jira base url: %w", err)
	}

	cleanPath := strings.TrimSpace(request.Path)
	if cleanPath == "" {
		return tracker.RawResponse{}, errors.New("jira request path is required")
	}
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}
	if !strings.HasPrefix(cleanPath, "/rest/api/3/") {
		return tracker.RawResponse{}, errors.New("jira_api only allows /rest/api/3/* paths")
	}
	endpoint.Path = path.Join(endpoint.Path, cleanPath)

	query := endpoint.Query()
	for key, value := range request.Query {
		query.Set(key, value)
	}
	endpoint.RawQuery = query.Encode()

	var bodyReader io.Reader
	if request.Body != nil {
		payload, err := json.Marshal(request.Body)
		if err != nil {
			return tracker.RawResponse{}, fmt.Errorf("encode jira request body: %w", err)
		}
		bodyReader = bytes.NewReader(payload)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bodyReader)
	if err != nil {
		return tracker.RawResponse{}, fmt.Errorf("build jira request: %w", err)
	}

	client.applyAuth(httpRequest)
	if request.Body != nil {
		httpRequest.Header.Set("Content-Type", "application/json")
	}
	httpRequest.Header.Set("Accept", "application/json")

	httpResponse, err := client.httpClient.Do(httpRequest)
	if err != nil {
		return tracker.RawResponse{}, fmt.Errorf("jira request failed: %w", err)
	}
	defer httpResponse.Body.Close()

	responseBody, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return tracker.RawResponse{}, fmt.Errorf("read jira response body: %w", err)
	}

	result := tracker.RawResponse{
		StatusCode: httpResponse.StatusCode,
		Headers:    httpResponse.Header,
		RawBody:    string(responseBody),
	}

	if len(responseBody) > 0 {
		var decoded any
		if err := json.Unmarshal(responseBody, &decoded); err == nil {
			result.Body = decoded
		} else {
			result.Body = string(responseBody)
		}
	}

	return result, nil
}

func (client *Client) searchIssues(ctx context.Context, jql string) ([]domain.Issue, error) {
	issues := make([]domain.Issue, 0)
	startAt := 0
	pageSize := 50

	for {
		response, err := client.RawAPI(ctx, tracker.RawRequest{
			Method: http.MethodGet,
			Path:   "/rest/api/3/search",
			Query: map[string]string{
				"jql":        jql,
				"startAt":    strconv.Itoa(startAt),
				"maxResults": strconv.Itoa(pageSize),
				"fields":     strings.Join(defaultSearchFields(), ","),
			},
		})
		if err != nil {
			return nil, err
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("jira search failed with status %d", response.StatusCode)
		}

		payload, ok := response.Body.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("jira search response was not an object")
		}
		pageIssues, total, nextStart, err := client.decodeSearchPage(payload, startAt)
		if err != nil {
			return nil, err
		}
		issues = append(issues, pageIssues...)
		if nextStart >= total || len(pageIssues) == 0 {
			break
		}
		startAt = nextStart
	}

	domain.SortIssues(issues)
	return issues, nil
}

func (client *Client) fetchIssue(ctx context.Context, id string) (domain.Issue, error) {
	response, err := client.RawAPI(ctx, tracker.RawRequest{
		Method: http.MethodGet,
		Path:   "/rest/api/3/issue/" + id,
		Query: map[string]string{
			"fields": strings.Join(defaultSearchFields(), ","),
		},
	})
	if err != nil {
		return domain.Issue{}, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return domain.Issue{}, fmt.Errorf("jira issue fetch failed with status %d", response.StatusCode)
	}
	payload, ok := response.Body.(map[string]any)
	if !ok {
		return domain.Issue{}, fmt.Errorf("jira issue response was not an object")
	}
	return mapIssue(payload), nil
}

func (client *Client) decodeSearchPage(payload map[string]any, startAt int) ([]domain.Issue, int, int, error) {
	total := intFromAny(payload["total"])
	if total == 0 {
		total = startAt
	}
	results, _ := payload["issues"].([]any)
	issues := make([]domain.Issue, 0, len(results))
	for _, raw := range results {
		issuePayload, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		issues = append(issues, mapIssue(issuePayload))
	}
	return issues, total, startAt + len(issues), nil
}

func (client *Client) defaultCandidateJQL() string {
	if strings.TrimSpace(client.cfg.Tracker.ProjectKey) == "" {
		return "ORDER BY created DESC"
	}
	states := quoteJQLStrings(client.cfg.Tracker.ActiveStates)
	return fmt.Sprintf("project = %s AND status in (%s) ORDER BY created ASC", client.cfg.Tracker.ProjectKey, states)
}

func (client *Client) jqlForStates(states []string) string {
	quotedStates := quoteJQLStrings(states)
	if strings.TrimSpace(client.cfg.Tracker.ProjectKey) == "" {
		return fmt.Sprintf("status in (%s) ORDER BY created ASC", quotedStates)
	}
	return fmt.Sprintf("project = %s AND status in (%s) ORDER BY created ASC", client.cfg.Tracker.ProjectKey, quotedStates)
}

func (client *Client) applyAuth(request *http.Request) {
	switch strings.ToLower(strings.TrimSpace(client.cfg.Tracker.AuthMode)) {
	case "bearer":
		request.Header.Set("Authorization", "Bearer "+client.cfg.Tracker.APIToken)
	default:
		credentials := client.cfg.Tracker.Email + ":" + client.cfg.Tracker.APIToken
		request.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	}
}

func mapIssue(payload map[string]any) domain.Issue {
	fields, _ := payload["fields"].(map[string]any)
	identifier, _ := payload["key"].(string)
	id, _ := payload["id"].(string)
	title, _ := fields["summary"].(string)
	statusName := nestedString(fields, "status", "name")
	branchName := nestedString(fields, "customfield_branch", "name")
	issueURL := ""
	if self, ok := payload["self"].(string); ok {
		issueURL = self
	}

	labels := make([]string, 0)
	if rawLabels, ok := fields["labels"].([]any); ok {
		for _, rawLabel := range rawLabels {
			if label, ok := rawLabel.(string); ok && strings.TrimSpace(label) != "" {
				labels = append(labels, strings.ToLower(strings.TrimSpace(label)))
			}
		}
	}

	description := descriptionMarkdown(fields["description"])
	priority := priorityFromFields(fields)
	createdAt := parseTime(nestedString(fields, "created"))
	updatedAt := parseTime(nestedString(fields, "updated"))

	return domain.Issue{
		ID:          id,
		Identifier:  identifier,
		Title:       title,
		Description: description,
		Priority:    priority,
		State:       statusName,
		BranchName:  branchName,
		URL:         issueURL,
		Labels:      labels,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
		Raw:         payload,
	}
}

func descriptionMarkdown(raw any) string {
	switch typed := raw.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(typed)
	default:
		return adfToMarkdown(raw)
	}
}

func priorityFromFields(fields map[string]any) *int {
	priorityMap, _ := fields["priority"].(map[string]any)
	if priorityMap == nil {
		return nil
	}
	priorityID, _ := priorityMap["id"].(string)
	if priorityID == "" {
		return nil
	}
	value, err := strconv.Atoi(priorityID)
	if err != nil {
		return nil
	}
	return &value
}

func nestedString(root any, path ...string) string {
	current := root
	for _, key := range path {
		mapValue, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current = mapValue[key]
	}
	text, _ := current.(string)
	return text
}

func parseTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05.999-0700"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			utc := parsed.UTC()
			return &utc
		}
	}
	return nil
}

func quoteJQLStrings(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		quoted = append(quoted, fmt.Sprintf("\"%s\"", strings.ReplaceAll(trimmed, "\"", "\\\"")))
	}
	if len(quoted) == 0 {
		return "\"\""
	}
	return strings.Join(quoted, ",")
}

func defaultSearchFields() []string {
	return []string{"summary", "description", "status", "priority", "labels", "created", "updated"}
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func plainTextADF(body string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": body,
					},
				},
			},
		},
	}
}
