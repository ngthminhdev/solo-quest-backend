package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/testutils"
)

func TestLearningRoadmapE2E_FullFlow(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	userID := testutils.BootstrapTestUser(t, db)
	r := testutils.CreateTestRouter(t, db, userID)

	// Seed roadmap data directly
	roadmap := &models.LearningRoadmap{
		Title: "Flutter App Architecture", Description: "Learn Flutter architecture", Category: "flutter",
		Difficulty: "normal", EstimatedMinutes: 180, TotalSteps: 3, Source: "system", Enabled: true,
	}
	db.Create(roadmap)
	step1 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "State Management", OrderIndex: 1, EstimatedMinutes: 30, Enabled: true}
	step2 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Dependency Injection", OrderIndex: 2, EstimatedMinutes: 25, Enabled: true}
	step3 := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Clean Architecture", OrderIndex: 3, EstimatedMinutes: 45, Enabled: true}
	db.Create(step1)
	db.Create(step2)
	db.Create(step3)

	send := func(method, path string, body interface{}) map[string]interface{} {
		var reqBody *bytes.Buffer
		if body != nil {
			jsonBody, _ := json.Marshal(body)
			reqBody = bytes.NewBuffer(jsonBody)
		} else {
			reqBody = bytes.NewBuffer(nil)
		}
		req, _ := http.NewRequest(method, path, reqBody)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		resp["_status"] = w.Code
		return resp
	}

	// 1. List roadmaps
	listResp := send("GET", "/api/learning-roadmaps", nil)
	if listResp["_status"].(int) != http.StatusOK {
		t.Fatalf("list failed: %v", listResp)
	}
	data := listResp["data"].(map[string]interface{})
	items := data["items"].([]interface{})
	if len(items) < 1 {
		t.Fatal("expected at least 1 roadmap")
	}

	// Find our roadmap
	var ourRoadmap map[string]interface{}
	for _, item := range items {
		rm := item.(map[string]interface{})
		if rm["title"].(string) == "Flutter App Architecture" {
			ourRoadmap = rm
			break
		}
	}
	if ourRoadmap == nil {
		t.Fatal("could not find seeded roadmap")
	}
	if ourRoadmap["status"].(string) != "" {
		t.Errorf("expected empty status before follow, got '%s'", ourRoadmap["status"])
	}
	if ourRoadmap["completed_steps"].(float64) != 0 {
		t.Errorf("expected 0 completed_steps before follow, got %v", ourRoadmap["completed_steps"])
	}

	// 2. Follow the roadmap
	followResp := send("POST", "/api/learning-roadmaps/"+roadmap.ID.String()+"/follow", nil)
	if followResp["_status"].(int) != http.StatusOK {
		t.Fatalf("follow failed: %v", followResp)
	}
	followData := followResp["data"].(map[string]interface{})
	if followData["status"].(string) != "tracking" {
		t.Errorf("expected status 'tracking', got '%s'", followData["status"])
	}

	// 3. Toggle first step complete
	toggleResp := send("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+step1.ID.String(), map[string]bool{"completed": true})
	if toggleResp["_status"].(int) != http.StatusOK {
		t.Fatalf("toggle failed: %v", toggleResp)
	}
	toggleData := toggleResp["data"].(map[string]interface{})
	if toggleData["completed"] != true {
		t.Error("expected completed=true")
	}
	if toggleData["completed_steps"].(float64) != 1 {
		t.Errorf("expected completed_steps=1, got %v", toggleData["completed_steps"])
	}
	if toggleData["progress_percent"].(float64) != 33 {
		t.Errorf("expected progress_percent=33, got %v", toggleData["progress_percent"])
	}
	if toggleData["roadmap_status"].(string) != "tracking" {
		t.Errorf("expected roadmap_status=tracking, got %v", toggleData["roadmap_status"])
	}

	// 4. Get detail to verify persistence
	detailResp := send("GET", "/api/learning-roadmaps/"+roadmap.ID.String(), nil)
	if detailResp["_status"].(int) != http.StatusOK {
		t.Fatalf("detail failed: %v", detailResp)
	}
	detailData := detailResp["data"].(map[string]interface{})
	if detailData["completed_steps"].(float64) != 1 {
		t.Errorf("expected completed_steps=1 in detail, got %v", detailData["completed_steps"])
	}
	if detailData["status"].(string) != "tracking" {
		t.Errorf("expected status=tracking in detail, got %v", detailData["status"])
	}
	detailSteps := detailData["steps"].([]interface{})
	firstStep := detailSteps[0].(map[string]interface{})
	if firstStep["completed"] != true {
		t.Error("first step should be completed in detail")
	}

	// 5. Toggle remaining steps to complete roadmap
	send("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+step2.ID.String(), map[string]bool{"completed": true})
	finalResp := send("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+step3.ID.String(), map[string]bool{"completed": true})
	if finalResp["_status"].(int) != http.StatusOK {
		t.Fatalf("final toggle failed: %v", finalResp)
	}
	finalData := finalResp["data"].(map[string]interface{})
	if finalData["roadmap_status"].(string) != "completed" {
		t.Errorf("expected roadmap_status=completed, got %v", finalData["roadmap_status"])
	}
	if finalData["progress_percent"].(float64) != 100 {
		t.Errorf("expected progress_percent=100, got %v", finalData["progress_percent"])
	}
	if finalData["completed_at"] == nil {
		t.Error("expected completed_at to be set")
	}

	// 6. Verify list now shows completed status
	listResp2 := send("GET", "/api/learning-roadmaps", nil)
	data2 := listResp2["data"].(map[string]interface{})
	items2 := data2["items"].([]interface{})
	for _, item := range items2 {
		rm := item.(map[string]interface{})
		if rm["id"].(string) == roadmap.ID.String() {
			if rm["status"].(string) != "completed" {
				t.Errorf("expected status=completed in list, got '%s'", rm["status"])
			}
			if rm["completed_steps"].(float64) != 3 {
				t.Errorf("expected completed_steps=3 in list, got %v", rm["completed_steps"])
			}
			break
		}
	}

	// 7. Uncheck a step and verify roadmap returns to tracking
	uncheckResp := send("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+step1.ID.String(), map[string]bool{"completed": false})
	if uncheckResp["_status"].(int) != http.StatusOK {
		t.Fatalf("uncheck failed: %v", uncheckResp)
	}
	uncheckData := uncheckResp["data"].(map[string]interface{})
	if uncheckData["roadmap_status"].(string) != "tracking" {
		t.Errorf("expected roadmap_status=tracking after uncheck, got %v", uncheckData["roadmap_status"])
	}
	if uncheckData["completed_at"] != nil {
		t.Error("expected completed_at=nil after uncheck")
	}
}

func TestLearningRoadmapE2E_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	defer testutils.CleanupTestDB(t, db)

	user1 := testutils.BootstrapTestUser(t, db)
	user2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	testutils.CreateTestUser(db, user2, "user2@test.com")

	roadmap := &models.LearningRoadmap{
		Title: "Test", Description: "desc", Category: "test",
		Difficulty: "normal", TotalSteps: 1, Source: "system", Enabled: true,
	}
	db.Create(roadmap)
	step := &models.LearningRoadmapStep{RoadmapID: roadmap.ID, Title: "Step 1", OrderIndex: 1, Enabled: true}
	db.Create(step)

	// User1 follows and completes
	r1 := testutils.CreateTestRouter(t, db, user1)
	send1 := func(method, path string, body interface{}) map[string]interface{} {
		var reqBody *bytes.Buffer
		if body != nil {
			jsonBody, _ := json.Marshal(body)
			reqBody = bytes.NewBuffer(jsonBody)
		} else {
			reqBody = bytes.NewBuffer(nil)
		}
		req, _ := http.NewRequest(method, path, reqBody)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r1.ServeHTTP(w, req)
		var resp map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &resp)
		resp["_status"] = w.Code
		return resp
	}

	send1("POST", "/api/learning-roadmaps/"+roadmap.ID.String()+"/follow", nil)
	send1("PATCH", "/api/learning-roadmaps/"+roadmap.ID.String()+"/steps/"+step.ID.String(), map[string]bool{"completed": true})

	// User2 should see no progress
	r2 := testutils.CreateTestRouter(t, db, user2)
	req2, _ := http.NewRequest("GET", "/api/learning-roadmaps/"+roadmap.ID.String(), nil)
	w2 := httptest.NewRecorder()
	r2.ServeHTTP(w2, req2)

	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	detailData := resp2["data"].(map[string]interface{})
	if detailData["completed_steps"].(float64) != 0 {
		t.Errorf("user2 should see 0 completed steps, got %v", detailData["completed_steps"])
	}
	if detailData["status"].(string) != "" {
		t.Errorf("user2 should see empty status, got '%s'", detailData["status"])
	}
}
