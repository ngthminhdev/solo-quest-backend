package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"solo_quest_backend/internal/handlers"
	"solo_quest_backend/internal/models"
	"solo_quest_backend/internal/services"
	"solo_quest_backend/internal/testutils"
)

func setupScheduleBlockRouter(t *testing.T) (*gin.Engine, uuid.UUID) {
	t.Helper()

	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userID := testutils.BootstrapTestUser(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	scheduleBlockService := services.NewScheduleBlockService(db)
	scheduleBlockHandler := handlers.NewScheduleBlockHandler(scheduleBlockService)

	protected := r.Group("")
	protected.Use(testutils.TestUserContext(userID))
	{
		sb := protected.Group("/api/schedule-blocks")
		{
			sb.GET("", scheduleBlockHandler.List)
			sb.POST("", scheduleBlockHandler.Create)
			sb.PUT("/:id", scheduleBlockHandler.Update)
			sb.PATCH("/:id", scheduleBlockHandler.Patch)
			sb.DELETE("/:id", scheduleBlockHandler.Delete)
		}
	}

	return r, userID
}

func createTestBlock(t *testing.T, r *gin.Engine) map[string]interface{} {
	t.Helper()

	body := map[string]interface{}{
		"title":        "Math Class",
		"type":         "school",
		"days_of_week": []int{1, 3, 5},
		"start_time":   "08:00",
		"end_time":     "09:30",
		"is_busy":      true,
		"is_flexible":  false,
		"location":     "Room 101",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("failed to create test block: %d %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp
}

func getBlockID(t *testing.T, resp map[string]interface{}) string {
	t.Helper()
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("response data is not an object")
	}
	id, ok := data["id"].(string)
	if !ok {
		t.Fatal("response data missing id")
	}
	return id
}

func TestScheduleBlock_ListEmpty(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	req, _ := http.NewRequest("GET", "/api/schedule-blocks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	code, _ := resp["code"].(float64)
	if int(code) != 200 {
		t.Errorf("expected code 200, got %v", code)
	}

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 0 {
		t.Errorf("expected empty array, got %d items", len(data))
	}
}

func TestScheduleBlock_ListNonEmpty(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	createTestBlock(t, r)

	req, _ := http.NewRequest("GET", "/api/schedule-blocks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 block, got %d", len(data))
	}
}

func TestScheduleBlock_CreateSuccess(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Math Class",
		"type":         "school",
		"days_of_week": []int{1, 3, 5},
		"start_time":   "08:00",
		"end_time":     "09:30",
		"is_busy":      true,
		"is_flexible":  false,
		"location":     "Room 101",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	code, _ := resp["code"].(float64)
	if int(code) != 201 {
		t.Errorf("expected code 201, got %v", code)
	}

	data := resp["data"].(map[string]interface{})
	if data["title"] != "Math Class" {
		t.Errorf("expected title 'Math Class', got '%v'", data["title"])
	}
	if data["type"] != "school" {
		t.Errorf("expected type 'school', got '%v'", data["type"])
	}
	if data["start_time"] != "08:00" {
		t.Errorf("expected start_time '08:00', got '%v'", data["start_time"])
	}
	if data["end_time"] != "09:30" {
		t.Errorf("expected end_time '09:30', got '%v'", data["end_time"])
	}
	if data["is_busy"] != true {
		t.Errorf("expected is_busy true, got %v", data["is_busy"])
	}
	if data["is_flexible"] != false {
		t.Errorf("expected is_flexible false, got %v", data["is_flexible"])
	}
	if data["enabled"] != true {
		t.Errorf("expected enabled true, got %v", data["enabled"])
	}
	if data["location"] != "Room 101" {
		t.Errorf("expected location 'Room 101', got '%v'", data["location"])
	}
	if _, ok := data["id"]; !ok {
		t.Error("response missing id field")
	}

	days, ok := data["days_of_week"].([]interface{})
	if !ok || len(days) != 3 {
		t.Errorf("expected 3 days_of_week, got %v", data["days_of_week"])
	}
}

func TestScheduleBlock_CreateWithDefaults(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Study Session",
		"type":         "study",
		"days_of_week": []int{2, 4},
		"start_time":   "19:00",
		"end_time":     "21:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})

	if data["is_busy"] != false {
		t.Errorf("expected is_busy default false, got %v", data["is_busy"])
	}
	if data["is_flexible"] != false {
		t.Errorf("expected is_flexible default false, got %v", data["is_flexible"])
	}
	if data["enabled"] != true {
		t.Errorf("expected enabled default true, got %v", data["enabled"])
	}
}

func TestScheduleBlock_CreateMissingTitle_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"type":         "school",
		"days_of_week": []int{1},
		"start_time":   "08:00",
		"end_time":     "09:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestScheduleBlock_CreateInvalidType_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Test",
		"type":         "invalid_type_xyz",
		"days_of_week": []int{1},
		"start_time":   "08:00",
		"end_time":     "09:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestScheduleBlock_CreateEmptyDaysOfWeek_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Test",
		"type":         "study",
		"days_of_week": []int{},
		"start_time":   "08:00",
		"end_time":     "09:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleBlock_CreateInvalidDayValue_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Test",
		"type":         "study",
		"days_of_week": []int{1, 8},
		"start_time":   "08:00",
		"end_time":     "09:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleBlock_CreateInvalidTimeRange_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Test",
		"type":         "study",
		"days_of_week": []int{1},
		"start_time":   "10:00",
		"end_time":     "10:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleBlock_PUTUpdateSuccess(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	created := createTestBlock(t, r)
	blockID := getBlockID(t, created)

	body := map[string]interface{}{
		"title":        "Advanced Math",
		"type":         "school",
		"days_of_week": []int{2, 4},
		"start_time":   "10:00",
		"end_time":     "11:30",
		"is_busy":      false,
		"is_flexible":  true,
		"location":     "Room 202",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/schedule-blocks/"+blockID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})

	if data["title"] != "Advanced Math" {
		t.Errorf("expected title 'Advanced Math', got '%v'", data["title"])
	}
	if data["start_time"] != "10:00" {
		t.Errorf("expected start_time '10:00', got '%v'", data["start_time"])
	}
	if data["end_time"] != "11:30" {
		t.Errorf("expected end_time '11:30', got '%v'", data["end_time"])
	}
	if data["is_busy"] != false {
		t.Errorf("expected is_busy false, got %v", data["is_busy"])
	}
	if data["is_flexible"] != true {
		t.Errorf("expected is_flexible true, got %v", data["is_flexible"])
	}
	if data["location"] != "Room 202" {
		t.Errorf("expected location 'Room 202', got '%v'", data["location"])
	}
}

func TestScheduleBlock_PUTNotFound_Returns404(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title":        "Test",
		"type":         "study",
		"days_of_week": []int{1},
		"start_time":   "08:00",
		"end_time":     "09:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PUT", "/api/schedule-blocks/"+uuid.New().String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestScheduleBlock_PATCHPartialUpdate(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	created := createTestBlock(t, r)
	blockID := getBlockID(t, created)

	body := map[string]interface{}{
		"title":        "Math 2.0",
		"is_flexible":  true,
		"location":     nil,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/schedule-blocks/"+blockID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})

	if data["title"] != "Math 2.0" {
		t.Errorf("expected title 'Math 2.0', got '%v'", data["title"])
	}
	if data["is_flexible"] != true {
		t.Errorf("expected is_flexible true, got %v", data["is_flexible"])
	}
	if data["start_time"] != "08:00" {
		t.Errorf("expected start_time preserved '08:00', got '%v'", data["start_time"])
	}
	if data["type"] != "school" {
		t.Errorf("expected type preserved 'school', got '%v'", data["type"])
	}
}

func TestScheduleBlock_PATCHOnlyEnabled(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	created := createTestBlock(t, r)
	blockID := getBlockID(t, created)

	body := map[string]interface{}{
		"enabled": false,
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/schedule-blocks/"+blockID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]interface{})

	if data["enabled"] != false {
		t.Errorf("expected enabled false, got %v", data["enabled"])
	}
}

func TestScheduleBlock_PATCHNotFound_Returns404(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	body := map[string]interface{}{
		"title": "Test",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/schedule-blocks/"+uuid.New().String(), bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestScheduleBlock_DeleteSuccess(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	created := createTestBlock(t, r)
	blockID := getBlockID(t, created)

	req, _ := http.NewRequest("DELETE", "/api/schedule-blocks/"+blockID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	code, _ := resp["code"].(float64)
	if int(code) != 200 {
		t.Errorf("expected code 200, got %v", code)
	}

	req2, _ := http.NewRequest("GET", "/api/schedule-blocks", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	var listResp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	data := listResp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected deleted block not in list, got %d items", len(data))
	}
}

func TestScheduleBlock_DeleteNotFound_Returns404(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	req, _ := http.NewRequest("DELETE", "/api/schedule-blocks/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestScheduleBlock_Unauthorized_Returns401(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	gin.SetMode(gin.TestMode)
	r := gin.New()

	scheduleBlockService := services.NewScheduleBlockService(db)
	handler := handlers.NewScheduleBlockHandler(scheduleBlockService)
	r.GET("/api/schedule-blocks", handler.List)

	req, _ := http.NewRequest("GET", "/api/schedule-blocks", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", w.Code)
	}
}

func TestScheduleBlock_UserIsolation(t *testing.T) {
	db := testutils.SetupTestDB(t)
	t.Cleanup(func() { testutils.CleanupTestDB(t, db) })

	userA := testutils.BootstrapTestUser(t, db)
	userB := uuid.New()
	testutils.CreateTestUser(db, userB, "userb@test.com")

	scheduleBlockService := services.NewScheduleBlockService(db)

	blockA := models.ScheduleBlock{
		UserID:    userA,
		Title:     "Block A",
		Type:      models.ScheduleBlockTypeStudy,
		StartTime: "10:00",
		EndTime:   "11:00",
		Enabled:   true,
	}
	blockA.DaysOfWeek, _ = json.Marshal([]int{1, 3})
	db.Create(&blockA)

	blockB := models.ScheduleBlock{
		UserID:    userB,
		Title:     "Block B",
		Type:      models.ScheduleBlockTypeWork,
		StartTime: "14:00",
		EndTime:   "15:00",
		Enabled:   true,
	}
	blockB.DaysOfWeek, _ = json.Marshal([]int{2, 4})
	db.Create(&blockB)

	gin.SetMode(gin.TestMode)
	handler := handlers.NewScheduleBlockHandler(scheduleBlockService)

	rA := gin.New()
	rA.GET("/api/schedule-blocks", testutils.AuthMiddleware(userA), handler.List)

	reqA, _ := http.NewRequest("GET", "/api/schedule-blocks", nil)
	wA := httptest.NewRecorder()
	rA.ServeHTTP(wA, reqA)

	var respA map[string]interface{}
	json.Unmarshal(wA.Body.Bytes(), &respA)
	dataA := respA["data"].([]interface{})
	if len(dataA) != 1 {
		t.Errorf("user A should see 1 block, got %d", len(dataA))
	} else {
		block := dataA[0].(map[string]interface{})
		if block["title"] != "Block A" {
			t.Errorf("user A should see 'Block A', got '%v'", block["title"])
		}
	}

	rB := gin.New()
	rB.GET("/api/schedule-blocks", testutils.AuthMiddleware(userB), handler.List)

	reqB, _ := http.NewRequest("GET", "/api/schedule-blocks", nil)
	wB := httptest.NewRecorder()
	rB.ServeHTTP(wB, reqB)

	var respB map[string]interface{}
	json.Unmarshal(wB.Body.Bytes(), &respB)
	dataB := respB["data"].([]interface{})
	if len(dataB) != 1 {
		t.Errorf("user B should see 1 block, got %d", len(dataB))
	} else {
		block := dataB[0].(map[string]interface{})
		if block["title"] != "Block B" {
			t.Errorf("user B should see 'Block B', got '%v'", block["title"])
		}
	}

	deleteReq, _ := http.NewRequest("DELETE", "/api/schedule-blocks/"+blockB.ID.String(), nil)
	rAA := gin.New()
	rAA.DELETE("/api/schedule-blocks/:id", testutils.AuthMiddleware(userA), handler.Delete)
	wDel := httptest.NewRecorder()
	rAA.ServeHTTP(wDel, deleteReq)

	if wDel.Code != http.StatusNotFound {
		t.Errorf("user A should not be able to delete user B's block, got status %d", wDel.Code)
	}
}

func TestScheduleBlock_ResponseEnvelope(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	created := createTestBlock(t, r)
	blockID := getBlockID(t, created)

	endpoints := []struct {
		method string
		path   string
		body   map[string]interface{}
		label  string
	}{
		{"GET", "/api/schedule-blocks", nil, "List"},
		{"PATCH", "/api/schedule-blocks/" + blockID, map[string]interface{}{"title": "X"}, "Patch"},
	}

	for _, ep := range endpoints {
		t.Run(ep.label, func(t *testing.T) {
			var jsonBody []byte
			if ep.body != nil {
				jsonBody, _ = json.Marshal(ep.body)
			}
			var req *http.Request
			if jsonBody != nil {
				req, _ = http.NewRequest(ep.method, ep.path, bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req, _ = http.NewRequest(ep.method, ep.path, nil)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			var resp map[string]interface{}
			json.Unmarshal(w.Body.Bytes(), &resp)

			if _, ok := resp["code"]; !ok {
				t.Errorf("%s: response missing 'code' field", ep.label)
			}
			if _, ok := resp["message"]; !ok {
				t.Errorf("%s: response missing 'message' field", ep.label)
			}
			if ep.method != "DELETE" {
				if _, ok := resp["data"]; !ok {
					t.Errorf("%s: response missing 'data' field", ep.label)
				}
			}
		})
	}
}

func TestScheduleBlock_PATCHTimeRangeValidation(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	created := createTestBlock(t, r)
	blockID := getBlockID(t, created)

	body := map[string]interface{}{
		"start_time": "10:00",
		"end_time":   "10:00",
	}
	jsonBody, _ := json.Marshal(body)

	req, _ := http.NewRequest("PATCH", "/api/schedule-blocks/"+blockID, bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid time range, got %d: %s", w.Code, w.Body.String())
	}
}

func TestScheduleBlock_CreateInvalidJSON_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	req, _ := http.NewRequest("POST", "/api/schedule-blocks", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestScheduleBlock_InvalidUUID_Returns400(t *testing.T) {
	r, _ := setupScheduleBlockRouter(t)

	req, _ := http.NewRequest("PATCH", "/api/schedule-blocks/not-a-uuid", bytes.NewReader([]byte(`{"title":"X"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
