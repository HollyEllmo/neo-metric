package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const (
	baseURL   = "http://localhost:8080/api/v1"
	accountID = "7" // cyber.uz
	imageURL  = "https://s3.sevendev.uz/local/2025/12/24/0eb0ad1e-9f02-4f69-bbf1-ec57b82939bf.png"
)

type CreatePublicationRequest struct {
	AccountID   string      `json:"account_id"`
	Type        string      `json:"type"`
	Caption     string      `json:"caption"`
	Media       []MediaItem `json:"media"`
	ScheduledAt *string     `json:"scheduled_at,omitempty"`
}

type UpdatePublicationRequest struct {
	Caption       *string     `json:"caption,omitempty"`
	Media         []MediaItem `json:"media,omitempty"`
	ScheduledAt   *string     `json:"scheduled_at,omitempty"`
	ClearSchedule bool        `json:"clear_schedule,omitempty"`
}

type ScheduleRequest struct {
	ScheduledAt string `json:"scheduled_at"`
}

type MediaItem struct {
	URL   string `json:"url"`
	Type  string `json:"type"`
	Order int    `json:"order"`
}

type Publication struct {
	ID               string      `json:"id"`
	AccountID        string      `json:"account_id"`
	InstagramMediaID string      `json:"instagram_media_id,omitempty"`
	Type             string      `json:"type"`
	Status           string      `json:"status"`
	Caption          string      `json:"caption"`
	Media            []MediaItem `json:"media,omitempty"`
	ScheduledAt      *string     `json:"scheduled_at,omitempty"`
}

type ListResponse struct {
	Publications []Publication `json:"publications"`
	Total        int64         `json:"total"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
}

// Helper function to create a test publication
func createTestPublication(t *testing.T, caption string) Publication {
	t.Helper()

	createReq := CreatePublicationRequest{
		AccountID: accountID,
		Type:      "post",
		Caption:   caption,
		Media: []MediaItem{
			{
				URL:   imageURL,
				Type:  "image",
				Order: 0,
			},
		},
	}

	body, _ := json.Marshal(createReq)
	resp, err := http.Post(baseURL+"/publications", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create publication: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(respBody))
	}

	var pub Publication
	if err := json.NewDecoder(resp.Body).Decode(&pub); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	return pub
}

// Helper function to delete a publication
func deleteTestPublication(t *testing.T, id string) {
	t.Helper()

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/publications/%s", baseURL, id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Logf("Warning: Failed to delete publication %s: %v", id, err)
		return
	}
	defer resp.Body.Close()
}

// TestPublicationCreate tests POST /publications
func TestPublicationCreate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("create post with single image", func(t *testing.T) {
		pub := createTestPublication(t, "Test create post #e2e")
		defer deleteTestPublication(t, pub.ID)

		if pub.ID == "" {
			t.Error("Expected ID to be set")
		}
		if pub.Status != "draft" {
			t.Errorf("Expected status 'draft', got '%s'", pub.Status)
		}
		if pub.AccountID != accountID {
			t.Errorf("Expected account_id '%s', got '%s'", accountID, pub.AccountID)
		}

		t.Logf("Created publication: ID=%s, Status=%s", pub.ID, pub.Status)
	})

	t.Run("create scheduled post", func(t *testing.T) {
		scheduledAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		createReq := CreatePublicationRequest{
			AccountID:   accountID,
			Type:        "post",
			Caption:     "Scheduled post #e2e",
			ScheduledAt: &scheduledAt,
			Media: []MediaItem{
				{URL: imageURL, Type: "image", Order: 0},
			},
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(baseURL+"/publications", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to create publication: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 201, got %d: %s", resp.StatusCode, string(respBody))
		}

		var pub Publication
		json.NewDecoder(resp.Body).Decode(&pub)
		defer deleteTestPublication(t, pub.ID)

		if pub.Status != "scheduled" {
			t.Errorf("Expected status 'scheduled', got '%s'", pub.Status)
		}

		t.Logf("Created scheduled publication: ID=%s, Status=%s", pub.ID, pub.Status)
	})

	t.Run("create without account_id fails", func(t *testing.T) {
		createReq := CreatePublicationRequest{
			Type:    "post",
			Caption: "No account",
			Media:   []MediaItem{{URL: imageURL, Type: "image", Order: 0}},
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(baseURL+"/publications", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})

	t.Run("create without media fails", func(t *testing.T) {
		createReq := CreatePublicationRequest{
			AccountID: accountID,
			Type:      "post",
			Caption:   "No media",
			Media:     []MediaItem{},
		}

		body, _ := json.Marshal(createReq)
		resp, err := http.Post(baseURL+"/publications", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// TestPublicationGet tests GET /publications/{id}
func TestPublicationGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("get existing publication", func(t *testing.T) {
		pub := createTestPublication(t, "Test get #e2e")
		defer deleteTestPublication(t, pub.ID)

		resp, err := http.Get(fmt.Sprintf("%s/publications/%s", baseURL, pub.ID))
		if err != nil {
			t.Fatalf("Failed to get publication: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}

		var fetched Publication
		json.NewDecoder(resp.Body).Decode(&fetched)

		if fetched.ID != pub.ID {
			t.Errorf("Expected ID '%s', got '%s'", pub.ID, fetched.ID)
		}

		t.Logf("Fetched publication: ID=%s, Status=%s", fetched.ID, fetched.Status)
	})

	t.Run("get non-existent publication returns 404", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/publications/%s", baseURL, "non-existent-id"))
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})
}

// TestPublicationList tests GET /publications
func TestPublicationList(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("list all publications", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/publications")
		if err != nil {
			t.Fatalf("Failed to list publications: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}

		var listResp ListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		t.Logf("Listed %d publications (total: %d)", len(listResp.Publications), listResp.Total)
	})

	t.Run("list with account filter", func(t *testing.T) {
		resp, err := http.Get(fmt.Sprintf("%s/publications?account_id=%s", baseURL, accountID))
		if err != nil {
			t.Fatalf("Failed to list publications: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var listResp ListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		for _, pub := range listResp.Publications {
			if pub.AccountID != accountID {
				t.Errorf("Expected account_id '%s', got '%s'", accountID, pub.AccountID)
			}
		}

		t.Logf("Listed %d publications for account %s", len(listResp.Publications), accountID)
	})

	t.Run("list with status filter", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/publications?status=draft")
		if err != nil {
			t.Fatalf("Failed to list publications: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var listResp ListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		t.Logf("Listed %d draft publications", len(listResp.Publications))
	})

	t.Run("list with pagination", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/publications?limit=5&offset=0")
		if err != nil {
			t.Fatalf("Failed to list publications: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var listResp ListResponse
		json.NewDecoder(resp.Body).Decode(&listResp)

		if listResp.Limit != 5 {
			t.Errorf("Expected limit 5, got %d", listResp.Limit)
		}
		if listResp.Offset != 0 {
			t.Errorf("Expected offset 0, got %d", listResp.Offset)
		}

		t.Logf("Listed %d publications with limit=5", len(listResp.Publications))
	})
}

// TestPublicationUpdate tests PUT /publications/{id}
func TestPublicationUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("update caption", func(t *testing.T) {
		pub := createTestPublication(t, "Original caption #e2e")
		defer deleteTestPublication(t, pub.ID)

		newCaption := "Updated caption #e2e"
		updateReq := UpdatePublicationRequest{
			Caption: &newCaption,
		}

		body, _ := json.Marshal(updateReq)
		req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/publications/%s", baseURL, pub.ID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to update publication: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}

		var updated Publication
		json.NewDecoder(resp.Body).Decode(&updated)

		if updated.Caption != newCaption {
			t.Errorf("Expected caption '%s', got '%s'", newCaption, updated.Caption)
		}

		t.Logf("Updated publication: ID=%s, Caption=%s", updated.ID, updated.Caption)
	})
}

// TestPublicationDelete tests DELETE /publications/{id}
func TestPublicationDelete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("delete draft publication", func(t *testing.T) {
		pub := createTestPublication(t, "To be deleted #e2e")

		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/publications/%s", baseURL, pub.ID), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to delete publication: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNoContent {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 204, got %d: %s", resp.StatusCode, string(respBody))
		}

		// Verify it's deleted
		getResp, err := http.Get(fmt.Sprintf("%s/publications/%s", baseURL, pub.ID))
		if err != nil {
			t.Fatalf("Failed to verify deletion: %v", err)
		}
		defer getResp.Body.Close()

		if getResp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404 after delete, got %d", getResp.StatusCode)
		}

		t.Logf("Deleted publication: ID=%s", pub.ID)
	})

	t.Run("delete non-existent publication returns 404", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/publications/%s", baseURL, "non-existent-id"), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}
	})
}

// TestPublicationPublish tests POST /publications/{id}/publish
func TestPublicationPublish(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("publish draft to Instagram", func(t *testing.T) {
		pub := createTestPublication(t, "Рюкзак для фотографов")

		publishURL := fmt.Sprintf("%s/publications/%s/publish", baseURL, pub.ID)
		req, _ := http.NewRequest(http.MethodPost, publishURL, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}

		var published Publication
		json.NewDecoder(resp.Body).Decode(&published)

		if published.Status != "published" {
			t.Errorf("Expected status 'published', got '%s'", published.Status)
		}

		if published.InstagramMediaID == "" {
			t.Error("Expected InstagramMediaID to be set")
		}

		t.Logf("Published! ID=%s, Status=%s, InstagramMediaID=%s", published.ID, published.Status, published.InstagramMediaID)
	})
}

// TestPublicationSchedule tests POST /publications/{id}/schedule
func TestPublicationSchedule(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("schedule draft publication", func(t *testing.T) {
		pub := createTestPublication(t, "Test schedule #e2e")
		defer deleteTestPublication(t, pub.ID)

		scheduledAt := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
		scheduleReq := ScheduleRequest{
			ScheduledAt: scheduledAt,
		}

		body, _ := json.Marshal(scheduleReq)
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/publications/%s/schedule", baseURL, pub.ID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to schedule publication: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}

		var scheduled Publication
		json.NewDecoder(resp.Body).Decode(&scheduled)

		if scheduled.Status != "scheduled" {
			t.Errorf("Expected status 'scheduled', got '%s'", scheduled.Status)
		}

		t.Logf("Scheduled publication: ID=%s, Status=%s", scheduled.ID, scheduled.Status)
	})

	t.Run("schedule with past time fails", func(t *testing.T) {
		pub := createTestPublication(t, "Test past schedule #e2e")
		defer deleteTestPublication(t, pub.ID)

		pastTime := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
		scheduleReq := ScheduleRequest{
			ScheduledAt: pastTime,
		}

		body, _ := json.Marshal(scheduleReq)
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/publications/%s/schedule", baseURL, pub.ID), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to make request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
	})
}

// TestPublicationDraft tests POST /publications/{id}/draft
func TestPublicationDraft(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	t.Run("save scheduled as draft", func(t *testing.T) {
		// Create scheduled publication
		scheduledAt := time.Now().Add(24 * time.Hour).Format(time.RFC3339)
		createReq := CreatePublicationRequest{
			AccountID:   accountID,
			Type:        "post",
			Caption:     "Scheduled to draft #e2e",
			ScheduledAt: &scheduledAt,
			Media: []MediaItem{
				{URL: imageURL, Type: "image", Order: 0},
			},
		}

		body, _ := json.Marshal(createReq)
		createResp, err := http.Post(baseURL+"/publications", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("Failed to create scheduled publication: %v", err)
		}
		defer createResp.Body.Close()

		var pub Publication
		json.NewDecoder(createResp.Body).Decode(&pub)
		defer deleteTestPublication(t, pub.ID)

		if pub.Status != "scheduled" {
			t.Fatalf("Expected initial status 'scheduled', got '%s'", pub.Status)
		}

		// Save as draft
		req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/publications/%s/draft", baseURL, pub.ID), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to save as draft: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			t.Fatalf("Expected status 200, got %d: %s", resp.StatusCode, string(respBody))
		}

		var draft Publication
		json.NewDecoder(resp.Body).Decode(&draft)

		if draft.Status != "draft" {
			t.Errorf("Expected status 'draft', got '%s'", draft.Status)
		}

		t.Logf("Saved as draft: ID=%s, Status=%s", draft.ID, draft.Status)
	})
}
