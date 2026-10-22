package tripo3d

import (
	"context"
	"errors"
	"strings"
)

// TextToModelParams are the parameters for POST /v3/generation/text-to-model.
type TextToModelParams struct {
	Prompt string `json:"prompt"`

	// Model selects the generation model; see the ModelVersion constants.
	// When unset, TextToModel sends ModelVersionH31: the API currently
	// rejects generation requests that omit model.
	Model *string `json:"model,omitempty"`

	NegativePrompt  *string `json:"negative_prompt,omitempty"`
	ImageSeed       *int64  `json:"image_seed,omitempty"`
	ModelSeed       *int64  `json:"model_seed,omitempty"`
	TextureSeed     *int64  `json:"texture_seed,omitempty"`
	Texture         *bool   `json:"texture,omitempty"`
	PBR             *bool   `json:"pbr,omitempty"`
	TextureQuality  *string `json:"texture_quality,omitempty"`
	GeometryQuality *string `json:"geometry_quality,omitempty"`
	FaceLimit       *int64  `json:"face_limit,omitempty"`
	AutoSize        *bool   `json:"auto_size,omitempty"`

	// Quad outputs a quad mesh instead of triangles, which forces the
	// output format to FBX rather than GLB. Within the P series only
	// ModelVersionP2 accepts it.
	Quad *bool `json:"quad,omitempty"`

	SmartLowPoly  *bool   `json:"smart_low_poly,omitempty"`
	GenerateParts *bool   `json:"generate_parts,omitempty"`
	Compress      *string `json:"compress,omitempty"`
	ExportUV      *bool   `json:"export_uv,omitempty"`

	// ExportOrientation sets the forward axis of the exported model; see
	// the ExportOrientation constants.
	ExportOrientation *string `json:"export_orientation,omitempty"`

	// TextureVersion selects the texture model; see the TextureVersion
	// constants. The server picks the default for Model when unset.
	TextureVersion *string `json:"texture_version,omitempty"`

	// Delight removes baked-in lighting from the texture. The server
	// defaults to true; it only takes effect with TextureVersionV35.
	Delight *bool `json:"delight,omitempty"`

	Style *string `json:"style,omitempty"`

	// Extra carries forward-compatible fields not yet modeled above; they
	// are merged into the JSON payload alongside the typed fields.
	Extra map[string]interface{} `json:"-"`
}

// TextToModel calls POST /v3/generation/text-to-model and returns the
// resulting task_id.
func (c *Client) TextToModel(ctx context.Context, params TextToModelParams) (string, error) {
	if strings.TrimSpace(params.Prompt) == "" {
		return "", errors.New("tripo3d: TextToModel: Prompt is required")
	}
	if params.Model == nil {
		params.Model = String(ModelVersionH31)
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	return c.createTask(ctx, "/generation/text-to-model", body)
}

// ImageToModelParams are the parameters for POST /v3/generation/image-to-model.
type ImageToModelParams struct {
	// Input is the source image: a public URL, a file_token, or the
	// task_id of an earlier text-to-image or image-to-image task whose
	// output should be reused.
	Input FileDescriptor `json:"input"`

	// Model selects the generation model; see the ModelVersion constants.
	// When unset, ImageToModel sends ModelVersionH31: the API currently
	// rejects generation requests that omit model.
	Model *string `json:"model,omitempty"`

	EnableImageAutofix *bool   `json:"enable_image_autofix,omitempty"`
	ModelSeed          *int64  `json:"model_seed,omitempty"`
	TextureSeed        *int64  `json:"texture_seed,omitempty"`
	Texture            *bool   `json:"texture,omitempty"`
	PBR                *bool   `json:"pbr,omitempty"`
	TextureQuality     *string `json:"texture_quality,omitempty"`
	TextureAlignment   *string `json:"texture_alignment,omitempty"`
	GeometryQuality    *string `json:"geometry_quality,omitempty"`
	FaceLimit          *int64  `json:"face_limit,omitempty"`
	AutoSize           *bool   `json:"auto_size,omitempty"`
	Orientation        *string `json:"orientation,omitempty"`

	// Quad outputs a quad mesh instead of triangles, which forces the
	// output format to FBX rather than GLB. Within the P series only
	// ModelVersionP2 accepts it.
	Quad *bool `json:"quad,omitempty"`

	SmartLowPoly  *bool   `json:"smart_low_poly,omitempty"`
	GenerateParts *bool   `json:"generate_parts,omitempty"`
	Compress      *string `json:"compress,omitempty"`
	ExportUV      *bool   `json:"export_uv,omitempty"`

	// ExportOrientation sets the forward axis of the exported model; see
	// the ExportOrientation constants.
	ExportOrientation *string `json:"export_orientation,omitempty"`

	// TextureVersion selects the texture model; see the TextureVersion
	// constants. The server picks the default for Model when unset.
	TextureVersion *string `json:"texture_version,omitempty"`

	// Delight removes baked-in lighting from the texture. The server
	// defaults to true; it only takes effect with TextureVersionV35.
	Delight *bool `json:"delight,omitempty"`

	Style *string `json:"style,omitempty"`

	Extra map[string]interface{} `json:"-"`
}

// ImageToModel calls POST /v3/generation/image-to-model and returns the
// resulting task_id.
func (c *Client) ImageToModel(ctx context.Context, params ImageToModelParams) (string, error) {
	if params.Input.IsEmpty() {
		return "", errors.New("tripo3d: ImageToModel: Input is required (url, file_token, or task_id)")
	}
	if params.Model == nil {
		params.Model = String(ModelVersionH31)
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	return c.createTask(ctx, "/generation/image-to-model", body)
}

// MultiviewToModelParams are the parameters for
// POST /v3/generation/multiview-to-model.
type MultiviewToModelParams struct {
	// Views lists 2 to 4 images keyed by angle, in any order, and is the
	// recommended input form. The front view is mandatory.
	Views []ViewImage `json:"-"`

	// Inputs holds exactly 4 views in [front, left, back, right] order.
	// The front view is mandatory and at least 2 views must be supplied;
	// leave the others as a zero-value FileDescriptor to skip them.
	Inputs *[4]FileDescriptor `json:"-"`

	// InputTaskID reuses the 4-view output of a successful
	// image-to-multiview or edit-multiview task.
	//
	// Exactly one of Views, Inputs and InputTaskID must be set.
	InputTaskID *string `json:"-"`

	// Model selects the generation model; see the ModelVersion constants.
	// When unset, MultiviewToModel sends ModelVersionH31: the API currently
	// rejects generation requests that omit model.
	Model *string `json:"model,omitempty"`

	ModelSeed        *int64  `json:"model_seed,omitempty"`
	TextureSeed      *int64  `json:"texture_seed,omitempty"`
	Texture          *bool   `json:"texture,omitempty"`
	PBR              *bool   `json:"pbr,omitempty"`
	TextureQuality   *string `json:"texture_quality,omitempty"`
	GeometryQuality  *string `json:"geometry_quality,omitempty"`
	TextureAlignment *string `json:"texture_alignment,omitempty"`
	FaceLimit        *int64  `json:"face_limit,omitempty"`
	AutoSize         *bool   `json:"auto_size,omitempty"`
	Orientation      *string `json:"orientation,omitempty"`

	// Quad outputs a quad mesh instead of triangles, which forces the
	// output format to FBX rather than GLB. Within the P series only
	// ModelVersionP2 accepts it.
	Quad *bool `json:"quad,omitempty"`

	SmartLowPoly      *bool   `json:"smart_low_poly,omitempty"`
	GenerateParts     *bool   `json:"generate_parts,omitempty"`
	Compress          *string `json:"compress,omitempty"`
	ExportUV          *bool   `json:"export_uv,omitempty"`
	ExportOrientation *string `json:"export_orientation,omitempty"`

	// TextureVersion selects the texture model; see the TextureVersion
	// constants. The server picks the default for Model when unset.
	TextureVersion *string `json:"texture_version,omitempty"`

	// Delight removes baked-in lighting from the texture. The server
	// defaults to true; it only takes effect with TextureVersionV35.
	Delight *bool `json:"delight,omitempty"`

	Extra map[string]interface{} `json:"-"`
}

// MultiviewToModel calls POST /v3/generation/multiview-to-model and returns
// the resulting task_id.
func (c *Client) MultiviewToModel(ctx context.Context, params MultiviewToModelParams) (string, error) {
	set := 0
	for _, ok := range []bool{len(params.Views) > 0, params.Inputs != nil, params.InputTaskID != nil} {
		if ok {
			set++
		}
	}
	if set == 0 {
		return "", errors.New("tripo3d: MultiviewToModel: provide Views, Inputs ([front, left, back, right]) or InputTaskID")
	}
	if set > 1 {
		return "", errors.New("tripo3d: MultiviewToModel: Views, Inputs and InputTaskID are mutually exclusive")
	}
	if len(params.Views) > 0 {
		if err := validateViews(params.Views); err != nil {
			return "", errors.New("tripo3d: MultiviewToModel: Views: " + err.Error())
		}
	}
	if params.Inputs != nil {
		if params.Inputs[0].IsEmpty() {
			return "", errors.New("tripo3d: MultiviewToModel: the front view (Inputs[0]) is required")
		}
		supplied := 0
		for _, v := range params.Inputs {
			if !v.IsEmpty() {
				supplied++
			}
		}
		if supplied < 2 {
			return "", errors.New("tripo3d: MultiviewToModel: at least 2 views are required")
		}
	}
	if params.Model == nil {
		params.Model = String(ModelVersionH31)
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	switch {
	case len(params.Views) > 0:
		body["inputs"] = params.Views
	case params.Inputs != nil:
		body["inputs"] = params.Inputs
	default:
		body["inputs"] = []map[string]string{{"task_id": *params.InputTaskID}}
	}
	return c.createTask(ctx, "/generation/multiview-to-model", body)
}

// TextToImageParams are the parameters for POST /v3/generation/text-to-image.
//
// Several fields are only honoured by a subset of the ImageModel values;
// see the constant documentation for Quality, Background and AspectRatio.
type TextToImageParams struct {
	// Prompt is required unless Template is set. Chinese and English are
	// both supported; put the most important elements first and append a
	// negative prompt after "--no".
	Prompt string `json:"prompt,omitempty"`

	// Model selects the image model; see the ImageModel constants.
	Model *string `json:"model,omitempty"`

	// Size is either a resolution tier such as "2K" or exact pixels such
	// as "2048x2048". The accepted values differ per model.
	Size *string `json:"size,omitempty"`

	Quality      *string `json:"quality,omitempty"`
	Background   *string `json:"background,omitempty"`
	AspectRatio  *string `json:"aspect_ratio,omitempty"`
	OutputFormat *string `json:"output_format,omitempty"`

	// Watermark adds an AI-generated-content watermark. Only the seedream
	// models honour it: the banana models always embed an invisible
	// watermark that cannot be disabled, and chat_image has no watermark
	// control at all.
	Watermark *bool `json:"watermark,omitempty"`

	// Template applies a generation preset and makes Prompt optional; see
	// the ImageTemplate constants.
	Template *string `json:"template,omitempty"`

	Extra map[string]interface{} `json:"-"`
}

// TextToImage calls POST /v3/generation/text-to-image and returns the
// resulting task_id.
func (c *Client) TextToImage(ctx context.Context, params TextToImageParams) (string, error) {
	if strings.TrimSpace(params.Prompt) == "" && params.Template == nil {
		return "", errors.New("tripo3d: TextToImage: Prompt is required unless Template is set")
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	return c.createTask(ctx, "/generation/text-to-image", body)
}

// ImageToImageParams are the parameters for POST /v3/generation/image-to-image.
//
// Several fields are only honoured by a subset of the ImageModel values;
// see the constant documentation for Quality, Background and AspectRatio.
// Note that ImageModelSeedreamV5 is the only seedream model this endpoint
// accepts — seedream_v4 is text-to-image only.
type ImageToImageParams struct {
	// Input is the single reference image: a public URL, a file_token, or
	// the task_id of an earlier image generation task. Mutually exclusive
	// with Inputs.
	Input *FileDescriptor `json:"input,omitempty"`

	// Inputs holds multiple reference images, referenced from Prompt as
	// "image[1]", "image[2]" and so on. The ceiling depends on the model:
	// 4 for seedream, 10 for banana, 16 for chat_image. Mutually exclusive
	// with Input.
	Inputs []FileDescriptor `json:"inputs,omitempty"`

	// Prompt is the edit instruction. Required unless Template is set.
	Prompt *string `json:"prompt,omitempty"`

	// Model selects the image editing model; see the ImageModel constants.
	Model *string `json:"model,omitempty"`

	// Size is either a resolution tier such as "2K" or exact pixels such
	// as "2048x2048". The accepted values differ per model.
	Size *string `json:"size,omitempty"`

	Quality      *string `json:"quality,omitempty"`
	Background   *string `json:"background,omitempty"`
	AspectRatio  *string `json:"aspect_ratio,omitempty"`
	OutputFormat *string `json:"output_format,omitempty"`

	// Template applies an edit preset and makes Prompt optional; see the
	// ImageTemplate constants.
	Template *string `json:"template,omitempty"`

	Extra map[string]interface{} `json:"-"`
}

// ImageToImage calls POST /v3/generation/image-to-image (edit, style
// transfer, or multi-image fusion) and returns the resulting task_id.
func (c *Client) ImageToImage(ctx context.Context, params ImageToImageParams) (string, error) {
	if params.Input == nil && len(params.Inputs) == 0 {
		return "", errors.New("tripo3d: ImageToImage: Input or Inputs is required")
	}
	if params.Input != nil && len(params.Inputs) > 0 {
		return "", errors.New("tripo3d: ImageToImage: Input and Inputs are mutually exclusive")
	}
	if params.Prompt == nil && params.Template == nil {
		return "", errors.New("tripo3d: ImageToImage: Prompt is required unless Template is set")
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	return c.createTask(ctx, "/generation/image-to-image", body)
}

// ImageToMultiviewParams are the parameters for
// POST /v3/generation/image-to-multiview.
type ImageToMultiviewParams struct {
	// Input is the source image: a public URL, a file_token, or the
	// task_id of an earlier image generation task.
	Input FileDescriptor `json:"input"`

	Extra map[string]interface{} `json:"-"`
}

// ImageToMultiview calls POST /v3/generation/image-to-multiview and returns
// the resulting task_id.
func (c *Client) ImageToMultiview(ctx context.Context, params ImageToMultiviewParams) (string, error) {
	if params.Input.IsEmpty() {
		return "", errors.New("tripo3d: ImageToMultiview: Input is required (url, file_token, or task_id)")
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	return c.createTask(ctx, "/generation/image-to-multiview", body)
}

// EditMultiviewParams are the parameters for
// POST /v3/generation/edit-multiview.
type EditMultiviewParams struct {
	// Input is the task_id of an earlier successful image-to-multiview or
	// edit-multiview task. The API documents file_token and URL inputs too,
	// but the service currently rejects anything that is not a task_id.
	Input FileDescriptor `json:"input"`

	// Prompts holds 1 to 4 per-view edit instructions.
	Prompts []MultiviewPrompt `json:"prompts"`

	Extra map[string]interface{} `json:"-"`
}

// EditMultiview calls POST /v3/generation/edit-multiview (apply per-view
// edits to a previously generated multiview set) and returns the resulting
// task_id.
func (c *Client) EditMultiview(ctx context.Context, params EditMultiviewParams) (string, error) {
	if params.Input.IsEmpty() {
		return "", errors.New("tripo3d: EditMultiview: Input is required (the task_id of a multiview task)")
	}
	if len(params.Prompts) == 0 || len(params.Prompts) > 4 {
		return "", errors.New("tripo3d: EditMultiview: Prompts must contain 1 to 4 items")
	}
	body, err := buildPayload(params, params.Extra)
	if err != nil {
		return "", err
	}
	return c.createTask(ctx, "/generation/edit-multiview", body)
}
