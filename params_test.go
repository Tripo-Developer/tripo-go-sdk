package tripo3d

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func taskCreated(w http.ResponseWriter) {
	writeJSON(w, 200, map[string]interface{}{"code": 0, "data": map[string]string{"task_id": "task_new"}})
}

func fileRef(ref string) *FileDescriptor {
	f := File(ref)
	return &f
}

func jsonValue(t *testing.T, raw string) interface{} {
	t.Helper()
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("bad fixture %s: %v", raw, err)
	}
	return v
}

func TestListTasksReturnsTasksKeyedByIDAndMissed(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tasks/list" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body := readJSONBody(t, r)
		if !reflect.DeepEqual(body["task_ids"], []interface{}{"task_a", "task_gone"}) {
			t.Fatalf("unexpected task_ids: %v", body["task_ids"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"status":"success","data":{
			"tasks":{"task_a":{"task_id":"task_a","type":"animate_rig","status":"success","progress":100,
				"output":{"model_url":"https://cdn.example/rig.fbx"},"credits_consumed":25}},
			"missed":["task_gone"]}}`))
	})

	list, err := client.ListTasks(context.Background(), []string{"task_a", "task_gone"})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	task, ok := list.Tasks["task_a"]
	if !ok || task.Status != TaskStatusSuccess || task.PrimaryModelURL() != "https://cdn.example/rig.fbx" {
		t.Fatalf("unexpected tasks: %+v", list.Tasks)
	}
	if !reflect.DeepEqual(list.Missed, []string{"task_gone"}) {
		t.Fatalf("unexpected missed: %v", list.Missed)
	}
}

func TestListTasksRejectsMoreThanTheServerLimit(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	ids := make([]string, MaxListTasks+1)
	for i := range ids {
		ids[i] = "task"
	}
	if _, err := client.ListTasks(context.Background(), ids); err == nil {
		t.Fatal("expected an error for more than MaxListTasks IDs")
	}
	if _, err := client.ListTasks(context.Background(), nil); err == nil {
		t.Fatal("expected an error for no IDs")
	}
}

func TestTextureModelSendsANestedTexturePrompt(t *testing.T) {
	var got map[string]interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = readJSONBody(t, r)
		taskCreated(w)
	})
	ctx := context.Background()

	if _, err := client.TextureModel(ctx, TextureModelParams{
		Input:         "task_src",
		Model:         String(TextureVersionV35),
		Delight:       Bool(false),
		TexturePrompt: &TexturePrompt{Text: "red paint", StyleImage: fileRef("file_style")},
	}); err != nil {
		t.Fatalf("TextureModel: %v", err)
	}
	want := jsonValue(t, `{"input":"task_src","model":"v3.5-20260815","delight":false,
		"texture_prompt":{"text":"red paint","style_image":"file_style"}}`)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected body: %v", got)
	}

	if _, err := client.TextureModel(ctx, TextureModelParams{
		Input: "task_src",
		TexturePrompt: &TexturePrompt{Images: []ViewImage{
			{View: ViewFront, Image: File("file_front")},
			{View: ViewBack, Image: FileURL("https://ex.com/back.png")},
		}},
	}); err != nil {
		t.Fatalf("TextureModel: %v", err)
	}
	want = jsonValue(t, `{"input":"task_src","texture_prompt":{"images":[
		{"front":"file_front"},{"back":{"url":"https://ex.com/back.png"}}]}}`)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected body: %v", got)
	}
}

func TestTextureModelMovesDeprecatedFieldsIntoTexturePrompt(t *testing.T) {
	var got map[string]interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = readJSONBody(t, r)
		taskCreated(w)
	})

	if _, err := client.TextureModel(context.Background(), TextureModelParams{
		Input:      "task_src",
		TextPrompt: String("red paint"),
		StyleImage: fileRef("file_style"),
	}); err != nil {
		t.Fatalf("TextureModel: %v", err)
	}
	want := jsonValue(t, `{"input":"task_src","texture_prompt":{"text":"red paint","style_image":"file_style"}}`)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected body: %v", got)
	}
}

func TestTextureModelRejectsInvalidTexturePrompts(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	cases := map[string]TextureModelParams{
		"text and image": {Input: "x", TexturePrompt: &TexturePrompt{Text: "a", Image: fileRef("f")}},
		"style without text": {Input: "x", TexturePrompt: &TexturePrompt{
			Image: fileRef("f"), StyleImage: fileRef("s"),
		}},
		"images without front": {Input: "x", TexturePrompt: &TexturePrompt{Images: []ViewImage{
			{View: ViewLeft, Image: File("l")}, {View: ViewBack, Image: File("b")},
		}}},
		"single image view": {Input: "x", TexturePrompt: &TexturePrompt{Images: []ViewImage{
			{View: ViewFront, Image: File("f")},
		}}},
		"deprecated with new":       {Input: "x", TextPrompt: String("a"), TexturePrompt: &TexturePrompt{Text: "b"}},
		"deprecated image and text": {Input: "x", TextPrompt: String("a"), ImagePrompt: fileRef("f")},
	}
	for name, params := range cases {
		if _, err := client.TextureModel(context.Background(), params); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestRigModelDefaultsToV30AndKeepsAnExplicitModel(t *testing.T) {
	var model interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		model = readJSONBody(t, r)["model"]
		taskCreated(w)
	})
	ctx := context.Background()

	for _, tc := range []struct {
		params RigModelParams
		want   string
	}{
		{RigModelParams{Input: "task_src"}, RigModelV30},
		{RigModelParams{Input: "task_src", Model: String(RigModelV10)}, RigModelV10},
		{RigModelParams{Input: "task_src", Extra: map[string]interface{}{"model": RigModelV25}}, RigModelV25},
	} {
		if _, err := client.RigModel(ctx, tc.params); err != nil {
			t.Fatalf("RigModel: %v", err)
		}
		if model != tc.want {
			t.Fatalf("expected model %s, got %v", tc.want, model)
		}
	}
}

func TestGenerationDefaultsToH31AndKeepsAnExplicitModel(t *testing.T) {
	var model interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		model = readJSONBody(t, r)["model"]
		taskCreated(w)
	})
	ctx := context.Background()
	views := []ViewImage{{View: ViewFront, Image: File("file_front")}, {View: ViewBack, Image: File("file_back")}}

	for _, tc := range []struct {
		name   string
		create func() (string, error)
		want   string
	}{
		{"text default", func() (string, error) {
			return client.TextToModel(ctx, TextToModelParams{Prompt: "a cat"})
		}, ModelVersionH31},
		{"text explicit", func() (string, error) {
			return client.TextToModel(ctx, TextToModelParams{Prompt: "a cat", Model: String(ModelVersionP1)})
		}, ModelVersionP1},
		{"image default", func() (string, error) {
			return client.ImageToModel(ctx, ImageToModelParams{Input: File("file_x")})
		}, ModelVersionH31},
		{"image extra", func() (string, error) {
			return client.ImageToModel(ctx, ImageToModelParams{Input: File("file_x"), Extra: map[string]interface{}{"model": ModelVersionH30}})
		}, ModelVersionH30},
		{"multiview default", func() (string, error) {
			return client.MultiviewToModel(ctx, MultiviewToModelParams{Views: views})
		}, ModelVersionH31},
	} {
		if _, err := tc.create(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if model != tc.want {
			t.Fatalf("%s: expected model %s, got %v", tc.name, tc.want, model)
		}
	}
}

func TestMultiviewToModelSendsViewKeyInputs(t *testing.T) {
	var got map[string]interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = readJSONBody(t, r)
		taskCreated(w)
	})

	if _, err := client.MultiviewToModel(context.Background(), MultiviewToModelParams{
		Views: []ViewImage{
			{View: ViewFront, Image: File("file_front")},
			{View: ViewRight, Image: File("file_right")},
		},
		TextureVersion: String(TextureVersionV35),
		TextureQuality: String(TextureQualityFast),
	}); err != nil {
		t.Fatalf("MultiviewToModel: %v", err)
	}
	want := jsonValue(t, `{"inputs":[{"front":"file_front"},{"right":"file_right"}],
		"model":"v3.1-20260211","texture_version":"v3.5-20260815","texture_quality":"fast"}`)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected body: %v", got)
	}
}

func TestMultiviewToModelRejectsInvalidViews(t *testing.T) {
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called")
	})
	front := ViewImage{View: ViewFront, Image: File("f")}
	cases := map[string]MultiviewToModelParams{
		"unknown view":   {Views: []ViewImage{front, {View: "top", Image: File("t")}}},
		"duplicate view": {Views: []ViewImage{front, front}},
		"empty image":    {Views: []ViewImage{front, {View: ViewLeft}}},
		"with inputs": {
			Views:  []ViewImage{front, {View: ViewLeft, Image: File("l")}},
			Inputs: &[4]FileDescriptor{File("f"), File("l")},
		},
	}
	for name, params := range cases {
		_, err := client.MultiviewToModel(context.Background(), params)
		if err == nil || !strings.HasPrefix(err.Error(), "tripo3d: MultiviewToModel:") {
			t.Errorf("%s: expected a MultiviewToModel error, got %v", name, err)
		}
	}
}

func TestNewPostProcessParamsAreSerialized(t *testing.T) {
	var got map[string]interface{}
	_, client := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		got = readJSONBody(t, r)
		taskCreated(w)
	})
	ctx := context.Background()

	cases := []struct {
		name string
		call func() (string, error)
		want string
	}{
		{"generation", func() (string, error) {
			return client.ImageToModel(ctx, ImageToModelParams{
				Input: File("file_img"), TextureVersion: String(TextureVersionV35), Delight: Bool(false),
			})
		}, `{"input":"file_img","model":"v3.1-20260211","texture_version":"v3.5-20260815","delight":false}`},
		{"convert", func() (string, error) {
			return client.ConvertModel(ctx, ConvertModelParams{
				Input: "task_src", Format: string(OutputFormatFBX),
				ExportVertexColors: Bool(true), ScaleFactor: Float64(0.01), AnimateInPlace: Bool(true),
				ExportOrientation: String(ExportOrientationMinusY), FBXPreset: String(FBXPresetBakeScale),
			})
		}, `{"input":"task_src","format":"FBX","export_vertex_colors":true,"scale_factor":0.01,
			"animate_in_place":true,"export_orientation":"-y","fbx_preset":"bake_scale"}`},
		{"segment", func() (string, error) {
			return client.SegmentMesh(ctx, SegmentMeshParams{
				Input: "task_src", Model: String(SegmentModelV2),
				SegmentationGranularity: String(SegmentationDetailed), SplitByConnectivity: Bool(false),
				RefImage: String("file_mask"),
			})
		}, `{"input":"task_src","model":"v2.0-20260430","segmentation_granularity":"detailed",
			"split_by_connectivity":false,"ref_image":"file_mask"}`},
		{"complete", func() (string, error) {
			return client.CompleteMesh(ctx, CompleteMeshParams{Input: "task_src", CompletionMode: String(CompletionQuickCap)})
		}, `{"input":"task_src","completion_mode":"quick_cap"}`},
		{"decimate", func() (string, error) {
			return client.DecimateMesh(ctx, DecimateMeshParams{
				Input: "task_src", Model: String(DecimateModelV30), SmartUV: Bool(true),
			})
		}, `{"input":"task_src","model":"v3.0","smart_uv":true}`},
	}
	for _, tc := range cases {
		if _, err := tc.call(); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if want := jsonValue(t, tc.want); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: unexpected body: %v", tc.name, got)
		}
	}
}
