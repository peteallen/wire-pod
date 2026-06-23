package wirepod_ttr

import (
	"errors"
	"strings"
	"testing"

	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
)

func preserveKnowledgeConfig(t *testing.T) {
	t.Helper()
	oldConfig := vars.APIConfig
	oldChats := vars.RememberedChats
	t.Cleanup(func() {
		vars.APIConfig = oldConfig
		vars.RememberedChats = oldChats
	})
}

func TestCreateAIReqUsesToolsAndModernPrompt(t *testing.T) {
	preserveKnowledgeConfig(t)
	vars.APIConfig.Knowledge.Provider = "openrouter"
	vars.APIConfig.Knowledge.Model = "x-ai/grok-4.3"
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.OpenAIPrompt = ""

	req := CreateAIReq("what do you see", "esn-1", false, false)

	if req.Model != "x-ai/grok-4.3" {
		t.Fatalf("model = %q", req.Model)
	}
	if req.MaxCompletionTokens != 2048 {
		t.Fatalf("MaxCompletionTokens = %d", req.MaxCompletionTokens)
	}
	if req.MaxTokens != 0 {
		t.Fatalf("MaxTokens = %d", req.MaxTokens)
	}
	if len(req.Tools) != 8 {
		t.Fatalf("tools length = %d", len(req.Tools))
	}
	if req.ToolChoice != "auto" {
		t.Fatalf("ToolChoice = %#v", req.ToolChoice)
	}
	if req.ParallelToolCalls != false {
		t.Fatalf("ParallelToolCalls = %#v", req.ParallelToolCalls)
	}

	prompt := req.Messages[0].Content
	if strings.Contains(prompt, "{{command||parameter}}") {
		t.Fatalf("modern tool prompt leaked legacy command syntax: %s", prompt)
	}
	if strings.Contains(prompt, "No special characters") {
		t.Fatalf("modern prompt still bans normal formatting with brittle text: %s", prompt)
	}
	if !strings.Contains(prompt, "action requires a tool call") {
		t.Fatalf("modern prompt did not explain that physical actions require tools: %s", prompt)
	}
	if !strings.Contains(prompt, "controlling Vector") {
		t.Fatalf("modern prompt did not explain the robot control context: %s", prompt)
	}
	if !strings.Contains(prompt, "set_eye_color") {
		t.Fatalf("modern prompt did not explain the eye color tool: %s", prompt)
	}
	if !strings.Contains(prompt, "taupe") || !strings.Contains(prompt, "top") {
		t.Fatalf("modern prompt did not guide color near-miss recovery: %s", prompt)
	}
	if !strings.Contains(prompt, "go_to_charger") {
		t.Fatalf("modern prompt did not explain the charger tool: %s", prompt)
	}
	if !strings.Contains(prompt, "run_action_sequence") || !strings.Contains(prompt, "drive in a circle") {
		t.Fatalf("modern prompt did not explain multi-step action sequences: %s", prompt)
	}
	if !strings.Contains(prompt, "run_builtin_intent") {
		t.Fatalf("modern prompt did not explain the built-in intent tool: %s", prompt)
	}
	if !strings.Contains(prompt, "intent_system_sleep") {
		t.Fatalf("modern prompt did not list built-in intents: %s", prompt)
	}
}

func TestCreateAIReqCanUseLegacyCommandFallbackPrompt(t *testing.T) {
	preserveKnowledgeConfig(t)
	vars.APIConfig.Knowledge.Provider = "custom"
	vars.APIConfig.Knowledge.Model = "local-model"
	vars.APIConfig.Knowledge.CommandsEnable = true

	req := createAIReqWithToolMode("animate", "esn-1", false, false, false)

	if len(req.Tools) != 0 {
		t.Fatalf("legacy fallback request included tools: %#v", req.Tools)
	}
	prompt := req.Messages[0].Content
	if !strings.Contains(prompt, "{{command||parameter}}") {
		t.Fatalf("legacy fallback prompt did not include command syntax: %s", prompt)
	}
	if !strings.Contains(prompt, "playAnimationWI") {
		t.Fatalf("legacy fallback prompt did not list command names: %s", prompt)
	}
}

func TestCreateAIReqKeepsConfiguredPromptAsPersonalityGuidance(t *testing.T) {
	preserveKnowledgeConfig(t)
	vars.APIConfig.Knowledge.Provider = "openrouter"
	vars.APIConfig.Knowledge.Model = "openai/gpt-oss-120b"
	vars.APIConfig.Knowledge.CommandsEnable = true
	vars.APIConfig.Knowledge.OpenAIPrompt = "You are cheerful and a little curious."

	req := CreateAIReq("say hi", "esn-1", false, false)
	prompt := req.Messages[0].Content

	if !strings.Contains(prompt, "Personality guidance from WirePod configuration: You are cheerful and a little curious.") {
		t.Fatalf("configured prompt was not preserved as personality guidance: %s", prompt)
	}
	if !strings.Contains(prompt, "does not replace the operating rules") {
		t.Fatalf("configured prompt was not separated from operating rules: %s", prompt)
	}
	if strings.Index(prompt, "Tool calls are executed by WirePod") > strings.Index(prompt, "Personality guidance") {
		t.Fatalf("operating rules should come before configurable personality guidance: %s", prompt)
	}
}

func TestLLMTurnAccumulatorKeepsFinalTextWithoutPunctuation(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "x-ai/grok-4.3")

	if err := acc.addRaw([]byte(`{"choices":[{"index":0,"delta":{"content":"Sure thing"},"finish_reason":null}]}`)); err != nil {
		t.Fatal(err)
	}
	result := acc.finish()

	if result.Content != "Sure thing" {
		t.Fatalf("Content = %q", result.Content)
	}
	if len(result.Segments) != 1 || result.Segments[0] != "Sure thing" {
		t.Fatalf("Segments = %#v", result.Segments)
	}
}

func TestLLMTurnAccumulatorKeepsPunctuatedSegmentsForSpeech(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "x-ai/grok-4.3")

	chunks := []string{
		`{"choices":[{"index":0,"delta":{"content":"The CCP was founded in 1921."},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"content":" It governs China today."},"finish_reason":"stop"}]}`,
	}
	for _, chunk := range chunks {
		if err := acc.addRaw([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	result := acc.finish()

	if result.Content != "The CCP was founded in 1921. It governs China today." {
		t.Fatalf("Content = %q", result.Content)
	}
	want := []string{"The CCP was founded in 1921.", "It governs China today."}
	if len(result.Segments) != len(want) {
		t.Fatalf("Segments = %#v", result.Segments)
	}
	for i := range want {
		if result.Segments[i] != want[i] {
			t.Fatalf("Segments[%d] = %q, want %q", i, result.Segments[i], want[i])
		}
	}
}

func TestLLMTurnAccumulatorRepairsLikelyMissingDeltaSpaces(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "openai/gpt-oss-120b")

	chunks := []string{
		`{"choices":[{"index":0,"delta":{"content":"The Chinese"},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"content":"Communist Party was founded in 1921. Could"},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"content":"you tell me the exact color?"},"finish_reason":"stop"}]}`,
	}
	for _, chunk := range chunks {
		if err := acc.addRaw([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	result := acc.finish()

	want := "The Chinese Communist Party was founded in 1921. Could you tell me the exact color?"
	if result.Content != want {
		t.Fatalf("Content = %q, want %q", result.Content, want)
	}
}

func TestLLMTurnAccumulatorRepairsAcronymBoundaryButKeepsCompactNames(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "openai/gpt-oss-120b")

	chunks := []string{
		`{"choices":[{"index":0,"delta":{"content":"The"},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"content":"CCP is discussed by Open"},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"content":"AI systems."},"finish_reason":"stop"}]}`,
	}
	for _, chunk := range chunks {
		if err := acc.addRaw([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	result := acc.finish()

	want := "The CCP is discussed by OpenAI systems."
	if result.Content != want {
		t.Fatalf("Content = %q, want %q", result.Content, want)
	}
}

func TestLLMTurnAccumulatorHandlesRefusalAsSpeakableText(t *testing.T) {
	acc := newLLMTurnAccumulator("openai", "gpt-5.5")

	if err := acc.addRaw([]byte(`{"choices":[{"index":0,"delta":{"refusal":"I cannot help with that."},"finish_reason":"stop"}]}`)); err != nil {
		t.Fatal(err)
	}
	result := acc.finish()

	if result.Content != "I cannot help with that." {
		t.Fatalf("Content = %q", result.Content)
	}
	if result.RefusalChars == 0 {
		t.Fatalf("RefusalChars was not recorded")
	}
}

func TestLLMTurnAccumulatorRecordsReasoningOnlyStream(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "openai/gpt-oss-120b")

	if err := acc.addRaw([]byte(`{"choices":[{"index":0,"delta":{"reasoning":"thinking"},"finish_reason":null}]}`)); err != nil {
		t.Fatal(err)
	}
	result := acc.finish()

	if result.Content != "" {
		t.Fatalf("Content = %q", result.Content)
	}
	if result.ReasoningChars == 0 {
		t.Fatalf("ReasoningChars was not recorded")
	}
}

func TestLLMTurnAccumulatorAggregatesToolCallDeltas(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "anthropic/claude-haiku-4.5")

	chunks := []string{
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"play_animation_non_interrupting","arguments":"{\"animation\""}}]},"finish_reason":null}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":":\"happy\"}"}}]},"finish_reason":"tool_calls"}]}`,
	}
	for _, chunk := range chunks {
		if err := acc.addRaw([]byte(chunk)); err != nil {
			t.Fatal(err)
		}
	}
	result := acc.finish()

	if result.FinishReason != "tool_calls" {
		t.Fatalf("FinishReason = %q", result.FinishReason)
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("ToolCalls = %#v", result.ToolCalls)
	}
	call := result.ToolCalls[0]
	if call.ID != "call_1" {
		t.Fatalf("tool call ID = %q", call.ID)
	}
	if call.Function.Name != "play_animation_non_interrupting" {
		t.Fatalf("tool call name = %q", call.Function.Name)
	}
	if call.Function.Arguments != `{"animation":"happy"}` {
		t.Fatalf("tool call arguments = %q", call.Function.Arguments)
	}
}

func TestEyeColorToolArgumentsNormalizeHexToHueSaturation(t *testing.T) {
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      robotToolSetEyeColor,
			Arguments: `{"hex_color":"#008080"}`,
		},
	}

	color, err := eyeColorArg(call)
	if err != nil {
		t.Fatal(err)
	}
	if color.Hex != "#008080" {
		t.Fatalf("Hex = %q", color.Hex)
	}
	if mathAbs(color.Hue-0.5) > 0.001 {
		t.Fatalf("Hue = %f", color.Hue)
	}
	if mathAbs(color.Saturation-1) > 0.001 {
		t.Fatalf("Saturation = %f", color.Saturation)
	}
}

func TestLegacySetEyeColorCommandFallback(t *testing.T) {
	action := CmdParamToAction("setEyeColor", "#E6E6FA")
	if action.Action != ActionSetEyeColor {
		t.Fatalf("Action = %d", action.Action)
	}
	if action.Parameter != "#E6E6FA" {
		t.Fatalf("Parameter = %q", action.Parameter)
	}
}

func TestLegacyGoToChargerCommandFallback(t *testing.T) {
	action := CmdParamToAction("goToCharger", "now")
	if action.Action != ActionGoToCharger {
		t.Fatalf("Action = %d", action.Action)
	}
	if action.Parameter != "now" {
		t.Fatalf("Parameter = %q", action.Parameter)
	}
}

func TestRobotIntentToolArgumentsValidateCatalog(t *testing.T) {
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      robotToolRunBuiltinIntent,
			Arguments: `{"intent":"intent_system_sleep"}`,
		},
	}

	intentName, err := robotIntentArg(call)
	if err != nil {
		t.Fatal(err)
	}
	if intentName != "intent_system_sleep" {
		t.Fatalf("intentName = %q", intentName)
	}
}

func TestToolCallReadinessSeparatesSpeechActionsFromDirectRobotCommands(t *testing.T) {
	if toolCallRequiresBehaviorReady(openai.ToolCall{Function: openai.FunctionCall{Name: robotToolGoToCharger}}) {
		t.Fatal("go_to_charger should execute directly without TTS behavior setup")
	}
	if toolCallRequiresBehaviorReady(openai.ToolCall{Function: openai.FunctionCall{Name: robotToolRunBuiltinIntent}}) {
		t.Fatal("run_builtin_intent should execute directly without TTS behavior setup")
	}
	if !toolCallRequiresBehaviorReady(openai.ToolCall{Function: openai.FunctionCall{Name: robotToolPlayAnimation}}) {
		t.Fatal("play_animation should use behavior setup")
	}
}

func TestLegacyRunBuiltinIntentCommandFallback(t *testing.T) {
	action := CmdParamToAction("runBuiltinIntent", "intent_imperative_dance")
	if action.Action != ActionRunBuiltinIntent {
		t.Fatalf("Action = %d", action.Action)
	}
	if action.Parameter != "intent_imperative_dance" {
		t.Fatalf("Parameter = %q", action.Parameter)
	}
}

func TestLLMTurnAccumulatorAcceptsEmptyUsageChoiceChunk(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "google/gemini-3.5-flash")

	if err := acc.addRaw([]byte(`{"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":2,"total_tokens":3}}`)); err != nil {
		t.Fatal(err)
	}
	result := acc.finish()

	if result.EmptyChoiceChunks != 1 {
		t.Fatalf("EmptyChoiceChunks = %d", result.EmptyChoiceChunks)
	}
}

func TestLLMTurnAccumulatorReturnsProviderError(t *testing.T) {
	acc := newLLMTurnAccumulator("openrouter", "x-ai/grok-4.3")

	err := acc.addRaw([]byte(`{"id":"cmpl-abc","error":{"code":"server_error","message":"Provider disconnected unexpectedly"},"choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}]}`))

	if err == nil {
		t.Fatal("expected provider error")
	}
	if !strings.Contains(err.Error(), "Provider disconnected unexpectedly") {
		t.Fatalf("error = %v", err)
	}
}

func TestLegacyCommandParserDoesNotPanicOnMalformedMarkup(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("GetActionsFromString panicked: %v", recovered)
		}
	}()

	actions := GetActionsFromString("hello {{broken}} world")

	if len(actions) == 0 {
		t.Fatal("expected fallback say-text action")
	}
}

func TestShouldRetryLLMWithoutTools(t *testing.T) {
	if !shouldRetryLLMWithoutTools(errors.New("unknown parameter: tools")) {
		t.Fatal("expected tools error to be retryable without tools")
	}
	if shouldRetryLLMWithoutTools(errors.New("invalid api key")) {
		t.Fatal("auth error should not be retried without tools")
	}
}

func TestRobotToolDefinitionsAreStrictFunctionTools(t *testing.T) {
	tools := robotToolDefinitions()
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		if tool.Type != openai.ToolTypeFunction {
			t.Fatalf("tool type = %q", tool.Type)
		}
		if tool.Function == nil {
			t.Fatalf("tool function was nil")
		}
		if !tool.Function.Strict {
			t.Fatalf("tool %s was not strict", tool.Function.Name)
		}
		toolNames[tool.Function.Name] = true
	}
	if !toolNames[robotToolGoToCharger] {
		t.Fatalf("missing %s tool", robotToolGoToCharger)
	}
	if !toolNames[robotToolRunBuiltinIntent] {
		t.Fatalf("missing %s tool", robotToolRunBuiltinIntent)
	}
	if !toolNames[robotToolRunActionSequence] {
		t.Fatalf("missing %s tool", robotToolRunActionSequence)
	}
	foundTaupeHint := false
	for _, tool := range tools {
		if tool.Function.Name != robotToolSetEyeColor {
			continue
		}
		params, ok := tool.Function.Parameters.(map[string]any)
		if !ok {
			t.Fatalf("set_eye_color parameters = %#v", tool.Function.Parameters)
		}
		props, ok := params["properties"].(map[string]any)
		if !ok {
			t.Fatalf("set_eye_color properties = %#v", params["properties"])
		}
		hex, ok := props["hex_color"].(map[string]any)
		if !ok {
			t.Fatalf("hex_color property = %#v", props["hex_color"])
		}
		desc, _ := hex["description"].(string)
		foundTaupeHint = strings.Contains(desc, "top") && strings.Contains(desc, "#483C32")
	}
	if !foundTaupeHint {
		t.Fatal("set_eye_color tool metadata did not include top-to-taupe guidance")
	}
	if !validRobotIntent("intent_system_sleep") {
		t.Fatal("expected sleep intent to be valid")
	}
}

func TestFirmwareToolsReturnIntentForVoiceStream(t *testing.T) {
	chargerResult := executeRobotToolCall(openai.ToolCall{
		Function: openai.FunctionCall{
			Name: robotToolGoToCharger,
		},
	}, nil, nil)

	if chargerResult.FirmwareIntent != "intent_system_charger" {
		t.Fatalf("go_to_charger firmware intent = %q", chargerResult.FirmwareIntent)
	}
	if !chargerResult.Disconnect {
		t.Fatal("go_to_charger should end the current turn after returning the firmware intent")
	}

	intentResult := executeRobotToolCall(openai.ToolCall{
		Function: openai.FunctionCall{
			Name:      robotToolRunBuiltinIntent,
			Arguments: `{"intent":"intent_play_popawheelie"}`,
		},
	}, nil, nil)

	if intentResult.FirmwareIntent != "intent_play_popawheelie" {
		t.Fatalf("run_builtin_intent firmware intent = %q", intentResult.FirmwareIntent)
	}
	if !intentResult.Disconnect {
		t.Fatal("run_builtin_intent should end the current turn after returning the firmware intent")
	}
}

func TestActionSequenceToolParsesCircleApproximation(t *testing.T) {
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Name: robotToolRunActionSequence,
			Arguments: `{"steps":[` +
				`{"action":"drive_straight","distance_mm":80,"speed_mmps":60},` +
				`{"action":"turn","angle_degrees":45},` +
				`{"action":"drive_straight","distance_mm":80,"speed_mmps":60},` +
				`{"action":"turn","angle_degrees":45}` +
				`]}`,
		},
	}

	steps, err := actionSequenceArg(call)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 4 {
		t.Fatalf("steps length = %d", len(steps))
	}
	if steps[0].Action != "drive_straight" || steps[1].Action != "turn" {
		t.Fatalf("steps = %#v", steps)
	}
}

func TestActionSequenceToolRejectsUnsafeLength(t *testing.T) {
	call := openai.ToolCall{
		Function: openai.FunctionCall{
			Name: robotToolRunActionSequence,
			Arguments: `{"steps":[` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100},` +
				`{"action":"wait","duration_ms":100}` +
				`]}`,
		},
	}

	if _, err := actionSequenceArg(call); err == nil {
		t.Fatal("expected overlong action sequence to be rejected")
	}
}

func TestDeliverFirmwareIntentUsesVoiceStreamEmitter(t *testing.T) {
	var gotIntent string
	var gotTool string
	err := deliverFirmwareIntent(robotToolExecution{
		Output:         "ok: heading to charger",
		FirmwareIntent: "intent_system_charger",
	}, robotToolGoToCharger, nil, func(intentName, toolName string) error {
		gotIntent = intentName
		gotTool = toolName
		return nil
	})

	if err != nil {
		t.Fatal(err)
	}
	if gotIntent != "intent_system_charger" {
		t.Fatalf("intent = %q", gotIntent)
	}
	if gotTool != robotToolGoToCharger {
		t.Fatalf("tool = %q", gotTool)
	}
}

func TestShouldBypassLocalIntentMatcherForLLMProviders(t *testing.T) {
	preserveKnowledgeConfig(t)
	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.IntentGraph = true
	vars.APIConfig.Knowledge.Provider = "openrouter"

	if !ShouldBypassLocalIntentMatcher() {
		t.Fatal("expected local intent matcher to be bypassed")
	}
	if ProcessTextAll(nil, "set your eyes teal", nil, false) {
		t.Fatal("expected ProcessTextAll to decline local matching")
	}
}

func TestShouldKeepLocalIntentMatcherForHoundify(t *testing.T) {
	preserveKnowledgeConfig(t)
	vars.APIConfig.Knowledge.Enable = true
	vars.APIConfig.Knowledge.IntentGraph = true
	vars.APIConfig.Knowledge.Provider = "houndify"

	if ShouldBypassLocalIntentMatcher() {
		t.Fatal("houndify should keep local matcher first")
	}
}

func mathAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
