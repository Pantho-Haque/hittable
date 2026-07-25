package requesteditor

import (
	"encoding/json"
	"testing"
)

func TestParamsJSONRoundTrip(t *testing.T) {
	p := NewParamsTab()

	params := map[string]string{
		"key":  "value",
		"key2": "value2",
	}
	p.SetContent(params)

	result := p.GetContent()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["key"] != "value" {
		t.Errorf("expected key=value, got %v", result["key"])
	}
	if result["key2"] != "value2" {
		t.Errorf("expected key2=value2, got %v", result["key2"])
	}
}

func TestParamsInvalidJSON(t *testing.T) {
	p := NewParamsTab()

	p.TextArea.SetValue("not valid json")
	result := p.GetContent()
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
	if !p.InvalidJSON {
		t.Error("expected InvalidJSON=true for invalid input")
	}
}

func TestParamsEmptyJSON(t *testing.T) {
	p := NewParamsTab()

	p.TextArea.SetValue("{}")
	result := p.GetContent()
	if result == nil {
		t.Fatal("expected non-nil for empty JSON object")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
	if p.InvalidJSON {
		t.Error("expected InvalidJSON=false for valid empty JSON")
	}
}

func TestParamsValidationDuringTyping(t *testing.T) {
	p := NewParamsTab()
	p.Focus()

	p.TextArea.SetValue(`{"key": "value"}`)
	p.Update(nil)

	if p.InvalidJSON {
		t.Error("expected valid JSON after setting valid content")
	}

	p.TextArea.SetValue(`{"key":`)
	p.Update(nil)

	if !p.InvalidJSON {
		t.Error("expected invalid JSON after setting partial content")
	}

	p.TextArea.SetValue(`{"key": "value"}`)
	p.Update(nil)

	if p.InvalidJSON {
		t.Error("expected valid JSON after fixing content")
	}
}

func TestHeadersJSONRoundTrip(t *testing.T) {
	h := NewHeadersTab()

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer token",
	}
	h.SetContent(headers)

	result := h.GetContent()
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type, got %v", result["Content-Type"])
	}
}

func TestHeadersInvalidJSON(t *testing.T) {
	h := NewHeadersTab()

	h.TextArea.SetValue("not valid json")
	result := h.GetContent()
	if result != nil {
		t.Errorf("expected nil for invalid JSON, got %v", result)
	}
	if !h.InvalidJSON {
		t.Error("expected InvalidJSON=true for invalid input")
	}
}

func TestParamsPrettyPrint(t *testing.T) {
	p := NewParamsTab()

	params := map[string]string{"key": "value"}
	p.SetContent(params)

	raw := p.TextArea.Value()
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if parsed["key"] != "value" {
		t.Errorf("expected key=value in pretty-printed JSON, got %v", parsed)
	}
}

func TestHeadersPrettyPrint(t *testing.T) {
	h := NewHeadersTab()

	headers := map[string]string{"Content-Type": "application/json"}
	h.SetContent(headers)

	raw := h.TextArea.Value()
	var parsed map[string]string
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error: %v", err)
	}
	if parsed["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type in pretty-printed JSON, got %v", parsed)
	}
}
