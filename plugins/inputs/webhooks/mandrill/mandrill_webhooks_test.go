package mandrill

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/shanas-swi/telegraf-v1.16.3/testutil"
)

func postWebhooks(md *MandrillWebhook, eventBody string) *httptest.ResponseRecorder {
	body := url.Values{}
	body.Set("mandrill_events", eventBody)
	req, _ := http.NewRequest("POST", "/mandrill", strings.NewReader(body.Encode()))
	w := httptest.NewRecorder()

	md.eventHandler(w, req)

	return w
}

func headRequest(md *MandrillWebhook) *httptest.ResponseRecorder {
	req, _ := http.NewRequest("HEAD", "/mandrill", strings.NewReader(""))
	w := httptest.NewRecorder()

	md.returnOK(w, req)

	return w
}

func TestHead(t *testing.T) {
	md := &MandrillWebhook{Path: "/mandrill"}
	resp := headRequest(md)
	if resp.Code != http.StatusOK {
		t.Errorf("HEAD returned HTTP status code %v.\nExpected %v", resp.Code, http.StatusOK)
	}
}

func TestSendEvent(t *testing.T) {
	var acc testutil.Accumulator
	md := &MandrillWebhook{Path: "/mandrill", acc: &acc}
	resp := postWebhooks(md, "["+SendEventJSON()+"]")
	if resp.Code != http.StatusOK {
		t.Errorf("POST send returned HTTP status code %v.\nExpected %v", resp.Code, http.StatusOK)
	}

	fields := map[string]interface{}{
		"id": "id1",
	}

	tags := map[string]string{
		"event": "send",
	}

	acc.AssertContainsTaggedFields(t, "mandrill_webhooks", fields, tags)
}

func TestMultipleEvents(t *testing.T) {
	var acc testutil.Accumulator
	md := &MandrillWebhook{Path: "/mandrill", acc: &acc}
	resp := postWebhooks(md, "["+SendEventJSON()+","+HardBounceEventJSON()+"]")
	if resp.Code != http.StatusOK {
		t.Errorf("POST send returned HTTP status code %v.\nExpected %v", resp.Code, http.StatusOK)
	}

	fields := map[string]interface{}{
		"id": "id1",
	}

	tags := map[string]string{
		"event": "send",
	}

	acc.AssertContainsTaggedFields(t, "mandrill_webhooks", fields, tags)

	fields = map[string]interface{}{
		"id": "id2",
	}

	tags = map[string]string{
		"event": "hard_bounce",
	}
	acc.AssertContainsTaggedFields(t, "mandrill_webhooks", fields, tags)
}
