package message

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewMessageManager_EmptyDirectory(t *testing.T) {
	// Test that an empty directory (no message_areas.json) results in 0 areas
	tempDir := t.TempDir()

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed with empty directory: %v", err)
	}

	areas := messageManager.ListAreas()
	if len(areas) != 0 {
		t.Errorf("expected 0 areas in empty directory, got %d", len(areas))
	}
}

func TestNewMessageManager_LoadsAreas(t *testing.T) {
	// Test that areas are loaded correctly from message_areas.json
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{
			ID:          1,
			Tag:         "GENERAL",
			Name:        "General Discussion",
			Description: "General chat area",
			ACSRead:     "",
			ACSWrite:    "",
		},
		{
			ID:          2,
			Tag:         "SYSOP",
			Name:        "SysOp Only",
			Description: "SysOp discussion area",
			ACSRead:     "S100",
			ACSWrite:    "S100",
		},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	areas := messageManager.ListAreas()
	if len(areas) != 2 {
		t.Errorf("expected 2 areas, got %d", len(areas))
	}

	// Verify areas are sorted by ID
	if len(areas) >= 2 {
		if areas[0].ID != 1 || areas[1].ID != 2 {
			t.Errorf("areas not sorted by ID: got IDs %d, %d", areas[0].ID, areas[1].ID)
		}
	}
}

func TestNewMessageManager_EmptyFile(t *testing.T) {
	// Test that an empty message_areas.json file is handled gracefully
	tempDir := t.TempDir()

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed with empty file: %v", err)
	}

	areas := messageManager.ListAreas()
	if len(areas) != 0 {
		t.Errorf("expected 0 areas from empty file, got %d", len(areas))
	}
}

func TestNewMessageManager_InvalidJSON(t *testing.T) {
	// Test that invalid JSON returns an error
	tempDir := t.TempDir()

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid message_areas.json: %v", err)
	}

	_, err := NewMessageManager(tempDir)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

func TestGetAreaByID(t *testing.T) {
	// Test retrieval of area by ID
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{ID: 1, Tag: "GENERAL", Name: "General Discussion"},
		{ID: 5, Tag: "TECH", Name: "Tech Talk"},
		{ID: 10, Tag: "SYSOP", Name: "SysOp Only"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Test finding existing area
	area, found := messageManager.GetAreaByID(5)
	if !found {
		t.Error("expected to find area with ID 5")
	}
	if area == nil {
		t.Fatal("area is nil but found is true")
	}
	if area.Tag != "TECH" {
		t.Errorf("expected tag 'TECH', got '%s'", area.Tag)
	}
	if area.Name != "Tech Talk" {
		t.Errorf("expected name 'Tech Talk', got '%s'", area.Name)
	}
}

func TestGetAreaByID_NotFound(t *testing.T) {
	// Test that non-existent area returns false
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{ID: 1, Tag: "GENERAL", Name: "General Discussion"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Test for non-existent ID
	area, found := messageManager.GetAreaByID(999)
	if found {
		t.Error("expected found to be false for non-existent ID")
	}
	if area != nil {
		t.Error("expected area to be nil for non-existent ID")
	}
}

func TestGetAreaByTag(t *testing.T) {
	// Test retrieval of area by Tag
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{ID: 1, Tag: "GENERAL", Name: "General Discussion"},
		{ID: 2, Tag: "SYSOP", Name: "SysOp Only"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Test finding existing area by tag
	area, found := messageManager.GetAreaByTag("SYSOP")
	if !found {
		t.Error("expected to find area with Tag 'SYSOP'")
	}
	if area == nil {
		t.Fatal("area is nil but found is true")
	}
	if area.ID != 2 {
		t.Errorf("expected ID 2, got %d", area.ID)
	}
	if area.Name != "SysOp Only" {
		t.Errorf("expected name 'SysOp Only', got '%s'", area.Name)
	}
}

func TestGetAreaByTag_NotFound(t *testing.T) {
	// Test that non-existent tag returns false
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{ID: 1, Tag: "GENERAL", Name: "General Discussion"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Test for non-existent tag
	area, found := messageManager.GetAreaByTag("NONEXISTENT")
	if found {
		t.Error("expected found to be false for non-existent tag")
	}
	if area != nil {
		t.Error("expected area to be nil for non-existent tag")
	}
}

func TestListAreas_SortedByID(t *testing.T) {
	// Test that ListAreas returns areas sorted by ID
	tempDir := t.TempDir()

	// Intentionally create areas in non-sorted order
	testAreas := []*MessageArea{
		{ID: 5, Tag: "AREA5", Name: "Area Five"},
		{ID: 1, Tag: "AREA1", Name: "Area One"},
		{ID: 10, Tag: "AREA10", Name: "Area Ten"},
		{ID: 3, Tag: "AREA3", Name: "Area Three"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	areas := messageManager.ListAreas()
	if len(areas) != 4 {
		t.Fatalf("expected 4 areas, got %d", len(areas))
	}

	expectedOrder := []int{1, 3, 5, 10}
	for i, expectedID := range expectedOrder {
		if areas[i].ID != expectedID {
			t.Errorf("position %d: expected ID %d, got %d", i, expectedID, areas[i].ID)
		}
	}
}

func TestDuplicateAreaID_SkipsSubsequent(t *testing.T) {
	// Test that duplicate area IDs result in only the first being loaded
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{ID: 1, Tag: "FIRST", Name: "First Area"},
		{ID: 1, Tag: "DUPLICATE", Name: "Duplicate Area"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	areas := messageManager.ListAreas()
	if len(areas) != 1 {
		t.Errorf("expected 1 area (duplicate skipped), got %d", len(areas))
	}

	if len(areas) > 0 && areas[0].Tag != "FIRST" {
		t.Errorf("expected first area to be kept, got tag '%s'", areas[0].Tag)
	}
}

func TestDuplicateAreaTag_SkipsSubsequent(t *testing.T) {
	// Test that duplicate area tags result in only the first being loaded
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{ID: 1, Tag: "SAME", Name: "First Area"},
		{ID: 2, Tag: "SAME", Name: "Duplicate Tag Area"},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	areas := messageManager.ListAreas()
	if len(areas) != 1 {
		t.Errorf("expected 1 area (duplicate tag skipped), got %d", len(areas))
	}

	if len(areas) > 0 && areas[0].Name != "First Area" {
		t.Errorf("expected first area to be kept, got name '%s'", areas[0].Name)
	}
}

func TestAddMessage(t *testing.T) {
	// Test adding a message to an area
	tempDir := t.TempDir()

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	testMessage := Message{
		ID:           uuid.New(),
		AreaID:       1,
		FromUserName: "TestUser",
		FromNodeID:   "NODE1",
		ToUserName:   MsgToUserAll,
		Subject:      "Test Subject",
		Body:         "Test message body",
		PostedAt:     time.Now(),
		IsPrivate:    false,
	}

	err = messageManager.AddMessage(1, testMessage)
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	// Verify the message file was created
	messageFilePath := filepath.Join(tempDir, "messages_area_1.jsonl")
	if _, err := os.Stat(messageFilePath); os.IsNotExist(err) {
		t.Error("expected message file to be created")
	}

	// Verify the message can be read back
	messages, err := messageManager.GetMessagesForArea(1, "")
	if err != nil {
		t.Fatalf("GetMessagesForArea failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Subject != "Test Subject" {
		t.Errorf("expected subject 'Test Subject', got '%s'", messages[0].Subject)
	}

	if messages[0].FromUserName != "TestUser" {
		t.Errorf("expected from user 'TestUser', got '%s'", messages[0].FromUserName)
	}
}

func TestGetMessagesForArea_NoFile(t *testing.T) {
	// Test getting messages when no message file exists
	tempDir := t.TempDir()

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	messages, err := messageManager.GetMessagesForArea(999, "")
	if err != nil {
		t.Fatalf("GetMessagesForArea should not error for non-existent file: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages for non-existent file, got %d", len(messages))
	}
}

func TestGetMessagesForArea_WithSinceMessageID(t *testing.T) {
	// Test filtering messages since a specific message ID
	tempDir := t.TempDir()

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Add multiple messages
	messageIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		messageIDs[i] = uuid.New()
		testMessage := Message{
			ID:           messageIDs[i],
			AreaID:       1,
			FromUserName: "TestUser",
			Subject:      "Test Subject " + string(rune('A'+i)),
			Body:         "Test body " + string(rune('A'+i)),
			PostedAt:     time.Now(),
		}
		if err := messageManager.AddMessage(1, testMessage); err != nil {
			t.Fatalf("AddMessage failed for message %d: %v", i, err)
		}
	}

	// Get messages since the first message (should return 2 messages)
	messages, err := messageManager.GetMessagesForArea(1, messageIDs[0].String())
	if err != nil {
		t.Fatalf("GetMessagesForArea with sinceMessageID failed: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("expected 2 messages after first message, got %d", len(messages))
	}

	// Get messages since the second message (should return 1 message)
	messages, err = messageManager.GetMessagesForArea(1, messageIDs[1].String())
	if err != nil {
		t.Fatalf("GetMessagesForArea with sinceMessageID failed: %v", err)
	}

	if len(messages) != 1 {
		t.Errorf("expected 1 message after second message, got %d", len(messages))
	}

	// Get messages since the last message (should return 0 messages)
	messages, err = messageManager.GetMessagesForArea(1, messageIDs[2].String())
	if err != nil {
		t.Fatalf("GetMessagesForArea with sinceMessageID failed: %v", err)
	}

	if len(messages) != 0 {
		t.Errorf("expected 0 messages after last message, got %d", len(messages))
	}
}

func TestGetMessageCountForArea(t *testing.T) {
	// Test counting messages in an area
	tempDir := t.TempDir()

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Initially should be 0
	count, err := messageManager.GetMessageCountForArea(1)
	if err != nil {
		t.Fatalf("GetMessageCountForArea failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 messages initially, got %d", count)
	}

	// Add some messages
	for i := 0; i < 5; i++ {
		testMessage := Message{
			ID:           uuid.New(),
			AreaID:       1,
			FromUserName: "TestUser",
			Subject:      "Test Subject",
			Body:         "Test body",
			PostedAt:     time.Now(),
		}
		if err := messageManager.AddMessage(1, testMessage); err != nil {
			t.Fatalf("AddMessage failed: %v", err)
		}
	}

	count, err = messageManager.GetMessageCountForArea(1)
	if err != nil {
		t.Fatalf("GetMessageCountForArea failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 messages, got %d", count)
	}
}

func TestGetNewMessageCount(t *testing.T) {
	// Test counting new messages since a specific message ID
	tempDir := t.TempDir()

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	// Add multiple messages
	messageIDs := make([]uuid.UUID, 5)
	for i := 0; i < 5; i++ {
		messageIDs[i] = uuid.New()
		testMessage := Message{
			ID:           messageIDs[i],
			AreaID:       1,
			FromUserName: "TestUser",
			Subject:      "Test Subject",
			Body:         "Test body",
			PostedAt:     time.Now(),
		}
		if err := messageManager.AddMessage(1, testMessage); err != nil {
			t.Fatalf("AddMessage failed: %v", err)
		}
	}

	// Count new messages since the second message (should be 3)
	count, err := messageManager.GetNewMessageCount(1, messageIDs[1].String())
	if err != nil {
		t.Fatalf("GetNewMessageCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 new messages, got %d", count)
	}

	// Count new messages since the last message (should be 0)
	count, err = messageManager.GetNewMessageCount(1, messageIDs[4].String())
	if err != nil {
		t.Fatalf("GetNewMessageCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 new messages after last, got %d", count)
	}

	// Count new messages with empty sinceMessageID (should return all 5)
	count, err = messageManager.GetNewMessageCount(1, "")
	if err != nil {
		t.Fatalf("GetNewMessageCount failed: %v", err)
	}
	if count != 5 {
		t.Errorf("expected 5 messages with empty sinceMessageID, got %d", count)
	}
}

func TestMessageAreaFields(t *testing.T) {
	// Test that all MessageArea fields are preserved through load/save cycle
	tempDir := t.TempDir()

	testAreas := []*MessageArea{
		{
			ID:            1,
			Tag:           "NETWORK",
			Name:          "Network Area",
			Description:   "A networked message area",
			ACSRead:       "S10",
			ACSWrite:      "S20",
			IsNetworked:   true,
			OriginNodeID:  "1:123/456",
			LastMessageID: "some-uuid-here",
		},
	}

	areasData, err := json.MarshalIndent(testAreas, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal test areas: %v", err)
	}

	areasPath := filepath.Join(tempDir, "message_areas.json")
	if err := os.WriteFile(areasPath, areasData, 0644); err != nil {
		t.Fatalf("failed to write test message_areas.json: %v", err)
	}

	messageManager, err := NewMessageManager(tempDir)
	if err != nil {
		t.Fatalf("NewMessageManager failed: %v", err)
	}

	area, found := messageManager.GetAreaByID(1)
	if !found {
		t.Fatal("area not found")
	}

	if area.ID != 1 {
		t.Errorf("expected ID 1, got %d", area.ID)
	}
	if area.Tag != "NETWORK" {
		t.Errorf("expected Tag 'NETWORK', got '%s'", area.Tag)
	}
	if area.Name != "Network Area" {
		t.Errorf("expected Name 'Network Area', got '%s'", area.Name)
	}
	if area.Description != "A networked message area" {
		t.Errorf("expected Description 'A networked message area', got '%s'", area.Description)
	}
	if area.ACSRead != "S10" {
		t.Errorf("expected ACSRead 'S10', got '%s'", area.ACSRead)
	}
	if area.ACSWrite != "S20" {
		t.Errorf("expected ACSWrite 'S20', got '%s'", area.ACSWrite)
	}
	if !area.IsNetworked {
		t.Error("expected IsNetworked to be true")
	}
	if area.OriginNodeID != "1:123/456" {
		t.Errorf("expected OriginNodeID '1:123/456', got '%s'", area.OriginNodeID)
	}
	if area.LastMessageID != "some-uuid-here" {
		t.Errorf("expected LastMessageID 'some-uuid-here', got '%s'", area.LastMessageID)
	}
}
