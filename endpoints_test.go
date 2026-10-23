package tripo3d

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestNewTaskEndpointsSendTheirPayloads(t *testing.T) {
	var got map[string]interface{}
	var path string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		got = readJSONBody(t, r)
		taskCreated(w)
	})
	ctx := context.Background()

	cases := []struct {
		name, path string
		call       func() (string, error)
		want       string
	}{
		{"splat", "/generation/image-to-splat", func() (string, error) {
			return client.ImageToSplat(ctx, ImageToSplatParams{Input: "file_img", ModelSeed: Int64(7)})
		}, `{"input":"file_img","model_seed":7}`},
		{"import", "/models/import", func() (string, error) {
			return client.ImportModel(ctx, ImportModelParams{Input: "https://ex.com/a.glb"})
		}, `{"input":"https://ex.com/a.glb"}`},
		{"smartsegment image", "/mesh/smartsegment", func() (string, error) {
			return client.SmartSegment(ctx, SmartSegmentParams{
				SegType: SegTypeImage, Input: "file_img", Granularity: String(SmartSegmentFine), Hint: String("sword"),
			})
		}, `{"seg_type":"image","input":"file_img","granularity":"fine","hint":"sword"}`},
		{"smartsegment model", "/mesh/smartsegment", func() (string, error) {
			return client.SmartSegment(ctx, SmartSegmentParams{SegType: SegTypeModel, Input: "file_glb", Transform: IdentityTransform()})
		}, `{"seg_type":"model","input":"file_glb","transform":[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1]}`},
	}
	for _, tc := range cases {
		if _, err := tc.call(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if path != tc.path {
			t.Errorf("%s: path %s, want %s", tc.name, path, tc.path)
		}
		if want := jsonValue(t, tc.want); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: unexpected body: %v", tc.name, got)
		}
	}
}

func TestSmartSegmentRejectsInvalidParamsWithoutCallingTheAPI(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request to %s", r.URL.Path)
	})
	ctx := context.Background()
	for name, params := range map[string]SmartSegmentParams{
		"no seg type":         {Input: "file_x"},
		"unknown seg type":    {SegType: "mesh", Input: "file_x"},
		"no input":            {SegType: SegTypeImage},
		"model w/o transform": {SegType: SegTypeModel, Input: "file_x"},
		"short transform":     {SegType: SegTypeModel, Input: "file_x", Transform: []float64{1, 0, 0}},
	} {
		if _, err := client.SmartSegment(ctx, params); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestSmartSegmentOutputIsThePrimaryModel(t *testing.T) {
	out := TaskOutput{SegModelURL: "https://cdn/seg.glb", MaskURL: "https://cdn/mask.png"}
	if got := out.PrimaryModelURL(); got != "https://cdn/seg.glb" {
		t.Fatalf("PrimaryModelURL = %q", got)
	}
}

func TestGetUsagePagesWithLimitAndOffset(t *testing.T) {
	var query string
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/account/usage" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		query = r.URL.RawQuery
		writeJSON(w, 200, map[string]interface{}{"code": 0, "data": []map[string]interface{}{{
			"task_id": "t1", "type": "animate_rig", "status": "success",
			"credits_consumed": 25, "created_at": "2026-10-20T09:35:19.372280Z",
		}}})
	})
	ctx := context.Background()

	records, err := client.GetUsage(ctx, UsageOptions{Limit: 5, Offset: 10})
	if err != nil {
		t.Fatalf("GetUsage: %v", err)
	}
	if query != "limit=5&offset=10" {
		t.Errorf("query = %q", query)
	}
	want := []UsageRecord{{TaskID: "t1", Type: "animate_rig", Status: TaskStatusSuccess, CreditsConsumed: 25, CreatedAt: "2026-10-20T09:35:19.372280Z"}}
	if !reflect.DeepEqual(records, want) {
		t.Errorf("records = %+v", records)
	}

	if _, err := client.GetUsage(ctx, UsageOptions{}); err != nil || query != "" {
		t.Errorf("default call: err=%v query=%q", err, query)
	}
	for _, opts := range []UsageOptions{{Limit: MaxUsageLimit + 1}, {Limit: -1}, {Offset: -1}} {
		if _, err := client.GetUsage(ctx, opts); err == nil {
			t.Errorf("%+v: expected an error", opts)
		}
	}
}

func TestUploadLargeFilePresignsThenPutsWithoutTheAPIKey(t *testing.T) {
	var put struct {
		auth, contentType string
		length            int64
		body              []byte
	}
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Fatalf("storage got %s", r.Method)
		}
		put.auth = r.Header.Get("Authorization")
		put.contentType = r.Header.Get("Content-Type")
		put.length = r.ContentLength
		put.body, _ = io.ReadAll(r.Body)
	}))
	t.Cleanup(storage.Close)

	var format interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/files/presign" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		format = readJSONBody(t, r)["format"]
		writeJSON(w, 200, map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"presigned_url": storage.URL + "/bucket/input.glb?X-Amz-Signature=abc",
			"file_token":    "file_big", "expires_in": 1800,
		}})
	})

	data := []byte("glTF-binary-bytes")
	uploaded, err := client.UploadLargeFile(context.Background(), bytes.NewReader(data), int64(len(data)), ".GLB")
	if err != nil {
		t.Fatalf("UploadLargeFile: %v", err)
	}
	if uploaded.FileToken != "file_big" || format != "glb" {
		t.Errorf("token=%q format=%v", uploaded.FileToken, format)
	}
	if put.auth != "" {
		t.Errorf("the PUT leaked an Authorization header: %q", put.auth)
	}
	if put.contentType != "application/octet-stream" || put.length != int64(len(data)) || !bytes.Equal(put.body, data) {
		t.Errorf("unexpected PUT: %+v", put)
	}
}

func TestUploadLargeFileSurfacesStorageErrors(t *testing.T) {
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, "<Error><Code>AccessDenied</Code></Error>")
	}))
	t.Cleanup(storage.Close)
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]interface{}{"code": 0, "data": map[string]interface{}{
			"presigned_url": storage.URL + "/x", "file_token": "file_big", "expires_in": 1800,
		}})
	})
	_, err := client.UploadLargeFile(context.Background(), strings.NewReader("abc"), 3, "glb")
	var reqErr *RequestError
	if !errors.As(err, &reqErr) || reqErr.StatusCode != http.StatusForbidden || !strings.Contains(reqErr.Body, "AccessDenied") {
		t.Fatalf("expected a 403 RequestError, got %v", err)
	}
}

// Signed with: printf '%s' "1714992000.<body>" | openssl dgst -sha256 -hmac whsec_test_secret
const (
	webhookSecret    = "whsec_test_secret"
	webhookBody      = `{"id":"task_abc123:task.completed","type":"task.completed","created_at":"2026-05-06T12:00:00Z","data":{"task_id":"task_abc123","type":"text_to_model","status":"success","output":{"model_url":"https://cdn.tripo3d.ai/output/model_pbr.glb"}}}`
	webhookSignature = "t=1714992000,v1=bcf3ecafcc87d73c1035ee9e31917393c8b15afcad1fba34b2fe4557db766b45"
)

func webhookAt(seconds int64) WebhookOptions {
	return WebhookOptions{Now: func() time.Time { return time.Unix(seconds, 0) }}
}

func TestVerifyWebhookAcceptsAValidSignature(t *testing.T) {
	event, err := VerifyWebhook([]byte(webhookBody), webhookSignature, webhookSecret, webhookAt(1714992000+60))
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event.Type != WebhookTaskCompleted || event.ID != "task_abc123:task.completed" {
		t.Errorf("event = %+v", event)
	}
	task, err := event.Task()
	if err != nil || task.TaskID != "task_abc123" || task.PrimaryModelURL() != "https://cdn.tripo3d.ai/output/model_pbr.glb" {
		t.Errorf("task = %+v, err = %v", task, err)
	}

	rotated := "t=1714992000,v1=00ff,v1=bcf3ecafcc87d73c1035ee9e31917393c8b15afcad1fba34b2fe4557db766b45"
	if _, err := VerifyWebhook([]byte(webhookBody), rotated, webhookSecret, webhookAt(1714992000)); err != nil {
		t.Errorf("a matching second v1 should pass: %v", err)
	}
	if _, err := VerifyWebhook([]byte(webhookBody), webhookSignature, webhookSecret, WebhookOptions{Tolerance: -1}); err != nil {
		t.Errorf("a negative tolerance should skip the timestamp check: %v", err)
	}
}

func TestVerifyWebhookRejectsTamperedOrStaleDeliveries(t *testing.T) {
	for name, tc := range map[string]struct {
		body, header, secret string
		opts                 WebhookOptions
	}{
		"tampered body":     {strings.Replace(webhookBody, "success", "failed", 1), webhookSignature, webhookSecret, webhookAt(1714992000)},
		"wrong secret":      {webhookBody, webhookSignature, "whsec_other", webhookAt(1714992000)},
		"stale":             {webhookBody, webhookSignature, webhookSecret, webhookAt(1714992000 + 301)},
		"from the future":   {webhookBody, webhookSignature, webhookSecret, webhookAt(1714992000 - 301)},
		"no timestamp":      {webhookBody, "v1=bcf3ecafcc87d73c1035ee9e31917393c8b15afcad1fba34b2fe4557db766b45", webhookSecret, webhookAt(1714992000)},
		"no signature":      {webhookBody, "t=1714992000", webhookSecret, webhookAt(1714992000)},
		"empty header":      {webhookBody, "", webhookSecret, webhookAt(1714992000)},
		"shifted timestamp": {webhookBody, strings.Replace(webhookSignature, "t=1714992000", "t=1714992001", 1), webhookSecret, webhookAt(1714992000)},
	} {
		if _, err := VerifyWebhook([]byte(tc.body), tc.header, tc.secret, tc.opts); !errors.Is(err, ErrWebhookSignature) {
			t.Errorf("%s: expected ErrWebhookSignature, got %v", name, err)
		}
	}
}
