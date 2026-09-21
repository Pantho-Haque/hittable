package hitfile

import (
    "encoding/json"
    "strings"
    "testing"
)
// making sure 
func TestRoundTrip(t *testing.T) {
    original := &HitFile{
        Method: "PATCH",
        URL:    "https://jsonplaceholder.typicode.com/posts/3",
        Headers: map[string]string{
            "Content-Type": "application/json",
        },
        Params: map[string]string{},
        Body:   `{"title": "Partially Updated"}`,
        Response: &Response{
            Data: map[string]interface{}{
                "userId": 1,
                "id":     3,
                "title":  "Partially Updated23",
            },
            Status:     200,
            StatusText: "OK",
            Ok:         true,
            Headers: map[string]string{
                "content-type": "application/json; charset=utf-8",
            },
            Cookies:    map[string]string{},
            DurationMs: 546,
            SizeBytes:  226,
        },
    }

    data := Marshal(original)
    if strings.Contains(string(data), "\\u003c") {
        t.Fatalf("Marshal must not HTML-escape: %s", data)
    }

    parsed, err := Parse(data)
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    if parsed.Method != original.Method {
        t.Errorf("Method: got %q, want %q", parsed.Method, original.Method)
    }
    if parsed.URL != original.URL {
        t.Errorf("URL: got %q, want %q", parsed.URL, original.URL)
    }
    if parsed.Body != original.Body {
        t.Errorf("Body: got %q, want %q", parsed.Body, original.Body)
    }
    if parsed.Response == nil {
        t.Fatal("Response is nil")
    }
    if parsed.Response.Status != original.Response.Status {
        t.Errorf("Status: got %d, want %d", parsed.Response.Status, original.Response.Status)
    }
    if parsed.Response.Ok != original.Response.Ok {
        t.Errorf("Ok: got %v, want %v", parsed.Response.Ok, original.Response.Ok)
    }

    data2 := Marshal(parsed)

    var obj1, obj2 interface{}
    json.Unmarshal(data, &obj1)
    json.Unmarshal(data2, &obj2)

    d1, _ := json.Marshal(obj1)
    d2, _ := json.Marshal(obj2)
    if string(d1) != string(d2) {
        t.Error("JSON round-trip produced different output")
    }
}

func TestParseEmptyResponse(t *testing.T) {
    input := `{
        "method": "GET",
        "url": "https://example.com",
        "headers": {},
        "params": {},
        "body": "",
        "response": null
    }`

    parsed, err := Parse([]byte(input))
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    if parsed.Response != nil {
        t.Error("expected nil response")
    }
}
