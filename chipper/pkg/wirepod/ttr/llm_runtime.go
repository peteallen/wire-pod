package wirepod_ttr

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
	"github.com/kercre123/wire-pod/chipper/pkg/logger"
	"github.com/kercre123/wire-pod/chipper/pkg/vars"
	"github.com/sashabaranov/go-openai"
)

const (
	robotToolPlayAnimation             = "play_animation"
	robotToolPlayAnimationNonInterrupt = "play_animation_non_interrupting"
	robotToolRequestNewVoiceInput      = "request_new_voice_input"
	robotToolCaptureImage              = "capture_image"
	robotToolSetEyeColor               = "set_eye_color"
	robotToolGoToCharger               = "go_to_charger"
	robotToolRunBuiltinIntent          = "run_builtin_intent"
	robotToolRunActionSequence         = "run_action_sequence"
	maxLLMToolTurns                    = 4
	llmChunkDebugEnv                   = "WIREPOD_LLM_DEBUG_CHUNKS"
	llmChunkDebugLegacyEnv             = "LLM_DEBUG_CHUNKS"
)

type promptMode int

const (
	promptModeTools promptMode = iota
	promptModeLegacyCommands
	promptModeNoCommands
)

type llmRuntimeResult struct {
	FinalText    string
	Messages     []openai.ChatCompletionMessage
	ToolActions  []string
	Disconnect   bool
	ReadyStarted bool
}

type llmTurnResult struct {
	Content           string
	Segments          []string
	ToolCalls         []openai.ToolCall
	FinishReason      string
	EmptyChoiceChunks int
	ReasoningChars    int
	RefusalChars      int
}

type llmProviderError struct {
	Code    any    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type llmStreamChunk struct {
	ID      string            `json:"id,omitempty"`
	Object  string            `json:"object,omitempty"`
	Model   string            `json:"model,omitempty"`
	Choices []llmStreamChoice `json:"choices,omitempty"`
	Error   *llmProviderError `json:"error,omitempty"`
	Usage   *openai.Usage     `json:"usage,omitempty"`
}

type llmStreamChoice struct {
	Index        int            `json:"index"`
	Delta        llmStreamDelta `json:"delta"`
	FinishReason string         `json:"finish_reason,omitempty"`
}

type llmStreamDelta struct {
	Role             string            `json:"role,omitempty"`
	Content          *string           `json:"content,omitempty"`
	Refusal          *string           `json:"refusal,omitempty"`
	Reasoning        *string           `json:"reasoning,omitempty"`
	ReasoningContent *string           `json:"reasoning_content,omitempty"`
	ReasoningDetails json.RawMessage   `json:"reasoning_details,omitempty"`
	ToolCalls        []openai.ToolCall `json:"tool_calls,omitempty"`
	FinishReason     string            `json:"finish_reason,omitempty"`
}

type toolCallAccumulator struct {
	order []int
	calls map[int]*toolCallState
}

type toolCallState struct {
	Index     int
	ID        string
	Type      openai.ToolType
	Name      string
	Arguments strings.Builder
}

type llmTurnAccumulator struct {
	provider          string
	model             string
	chunkCount        int
	emptyChoiceChunks int
	reasoningChars    int
	refusalChars      int
	finishReason      string
	segments          []string
	segmenter         sentenceSegmenter
	tools             toolCallAccumulator
}

type sentenceSegmenter struct {
	pending strings.Builder
	full    strings.Builder
}

type robotToolExecution struct {
	Output         string
	ImageMessage   *openai.ChatCompletionMessage
	FirmwareIntent string
	Disconnect     bool
}

func createPromptForMode(origPrompt, model string, isKG bool, mode promptMode) string {
	personalityPrompt := strings.TrimSpace(origPrompt)
	if personalityPrompt == "" {
		personalityPrompt = "Be a helpful, animated robot called Vector. Keep the response concise yet informative."
	}

	parts := []string{
		"You are controlling Vector, a real Anki Vector robot. Your assistant text is spoken aloud through Vector's voice. Tool calls are executed by WirePod against Vector's body, camera, settings, and firmware behaviors.",
		"Keep spoken replies friendly, natural, and concise. Normal punctuation is welcome because it helps speech sound natural. Avoid markdown, lists, emojis, and long paragraphs.",
		"User input comes from speech-to-text and may contain recognition mistakes. Infer the most likely request from context. If an eye-color command contains a color-like near miss, choose the most likely named color and call set_eye_color instead of asking. For example, 'top' in an eye-color request likely means taupe. If the request is genuinely ambiguous, ask one short clarifying question instead of guessing a physical action.",
	}

	switch mode {
	case promptModeTools:
		parts = append(parts,
			"Speech and action are separate. If the user asks you to do something with Vector's body, settings, camera, charger, cube, timers, or firmware behaviors, call the appropriate tool. Do not merely say that you did it. A short spoken acknowledgement is fine, but the action requires a tool call.",
			"Prefer specific tools when they fit exactly: play_animation_non_interrupting for expressive gestures while talking, play_animation when the animation itself is the requested action, go_to_charger for home/dock/charger requests, set_eye_color for eye color changes, capture_image for visual questions or new photos, and request_new_voice_input only when intentionally continuing the conversation after your reply.",
			"Use run_action_sequence for multi-step physical tasks that can be reasonably composed from short moves, turns, waits, animations, or eye-color changes. For example, approximate 'drive in a circle' with several short forward moves and small turns instead of saying you cannot do it.",
			"Use run_builtin_intent for Vector's built-in firmware commands that do not need custom arguments, including movement, sleep, greetings, cube games, praise or criticism reactions, volume changes, timer checks, dancing, tricks, message playback, and blackjack actions.",
			"Available built-in Vector intents for run_builtin_intent: "+robotIntentPromptCatalog()+".",
		)
	case promptModeLegacyCommands:
		parts = append(parts,
			"Tool calling is unavailable for this request, so robot actions must use the legacy command format {{command||parameter}}. Keep normal speech as plain text. If the user asks Vector to physically do something, include the matching command instead of only saying it was done.",
			legacyCommandPrompt(model),
		)
	}

	parts = append(parts,
		"Personality guidance from WirePod configuration: "+personalityPrompt,
		"The personality guidance shapes Vector's voice and character. It does not replace the operating rules above for tool use, safety, or robot capabilities.",
	)

	if mode != promptModeNoCommands {
		if isKG && vars.APIConfig.Knowledge.SaveChat {
			parts = append(parts, "Conversation mode is active. If you ask the user a follow-up question at the end of your response, request a new voice input.")
		} else {
			parts = append(parts, "Conversation mode is not active. Prefer answering directly and avoid asking follow-up questions unless the user clearly needs clarification.")
		}
	}

	prompt := strings.Join(parts, "\n\n")
	if os.Getenv("DEBUG_PRINT_PROMPT") == "true" {
		logger.Println(prompt)
	}
	return prompt
}

func legacyCommandPrompt(model string) string {
	var parts []string
	for _, cmd := range ValidLLMCommands {
		if !ModelIsSupported(cmd, model) {
			continue
		}
		parts = append(parts, "Command Name: "+cmd.Command+"\nDescription: "+cmd.Description+"\nParameter choices: "+cmd.ParamChoices)
	}
	return "Valid legacy commands:\n\n" + strings.Join(parts, "\n\n")
}

func robotToolDefinitions() []openai.Tool {
	animationSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"animation": map[string]any{
				"type":        "string",
				"description": "The Vector animation mood to play.",
				"enum":        animationNames(),
			},
		},
		"required":             []string{"animation"},
		"additionalProperties": false,
	}
	noArgsSchema := map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"required":             []string{},
		"additionalProperties": false,
	}
	captureSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"camera_angle": map[string]any{
				"type":        "string",
				"description": "Camera position to use for the photo.",
				"enum":        []string{"front", "lookingUp"},
			},
		},
		"required":             []string{"camera_angle"},
		"additionalProperties": false,
	}
	eyeColorSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hex_color": map[string]any{
				"type":        "string",
				"description": "The best RGB hex color for Vector's eyes, formatted as #RRGGBB. For example, teal is #008080 and taupe is #483C32. If speech-to-text hears an eye-color request as top, infer taupe and use #483C32.",
				"pattern":     "^#[0-9A-Fa-f]{6}$",
			},
		},
		"required":             []string{"hex_color"},
		"additionalProperties": false,
	}
	builtinIntentSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"intent": map[string]any{
				"type":        "string",
				"description": "The built-in Vector intent to run.",
				"enum":        robotIntentNames(),
			},
		},
		"required":             []string{"intent"},
		"additionalProperties": false,
	}
	actionSequenceSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"steps": map[string]any{
				"type":        "array",
				"description": "A bounded sequence of simple Vector actions. Use several short drive/turn steps to approximate paths like circles, squares, or patrols.",
				"minItems":    1,
				"maxItems":    8,
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type":        "string",
							"description": "The primitive action for this step.",
							"enum":        []string{"drive_straight", "turn", "play_animation", "play_animation_non_interrupting", "set_eye_color", "wait"},
						},
						"distance_mm": map[string]any{
							"type":        "number",
							"description": "Distance for drive_straight. Positive drives forward, negative backs up. Keep each step between -300 and 300.",
						},
						"speed_mmps": map[string]any{
							"type":        "number",
							"description": "Speed for drive_straight, from 20 to 100 mm/s. Defaults to 60.",
						},
						"angle_degrees": map[string]any{
							"type":        "number",
							"description": "Relative turn angle in degrees. Positive turns left, negative turns right. Keep each step between -180 and 180.",
						},
						"animation": map[string]any{
							"type":        "string",
							"description": "Animation mood for animation actions.",
							"enum":        animationNames(),
						},
						"hex_color": map[string]any{
							"type":        "string",
							"description": "Eye color for set_eye_color, formatted as #RRGGBB.",
							"pattern":     "^#[0-9A-Fa-f]{6}$",
						},
						"duration_ms": map[string]any{
							"type":        "integer",
							"description": "Wait duration for wait, from 100 to 2000 milliseconds.",
						},
					},
					"required":             []string{"action"},
					"additionalProperties": false,
				},
			},
		},
		"required":             []string{"steps"},
		"additionalProperties": false,
	}

	return []openai.Tool{
		functionTool(robotToolPlayAnimationNonInterrupt, "Play a short Vector animation without interrupting speech. Use for light emotional expression while Vector talks.", animationSchema),
		functionTool(robotToolPlayAnimation, "Play a short Vector animation that may interrupt speech. Use only when the user directly asks for an animation or the action should take priority over speech.", animationSchema),
		functionTool(robotToolGoToCharger, "Send Vector back to his charger. Use when the user asks Vector to go home, dock, charge, return to base, or get on the charger.", noArgsSchema),
		functionTool(robotToolSetEyeColor, "Set Vector's eye color. Use when the user asks to change Vector's eyes to a named color, custom color, or visual style.", eyeColorSchema),
		functionTool(robotToolRunActionSequence, "Run a bounded sequence of simple Vector actions for multi-step physical tasks. Use for requests like driving in a circle, making a square, patrolling, combining a gesture with movement, or similar composed actions.", actionSequenceSchema),
		functionTool(robotToolRunBuiltinIntent, "Run one of Vector's built-in firmware intents. Use for direct robot commands that do not need custom tool arguments.", builtinIntentSchema),
		functionTool(robotToolRequestNewVoiceInput, "Ask Vector to listen for another voice request after this response. Use only when intentionally continuing the conversation.", noArgsSchema),
		functionTool(robotToolCaptureImage, "Capture a new image from Vector's camera so the model can answer a question about what Vector sees.", captureSchema),
	}
}

func functionTool(name, description string, parameters map[string]any) openai.Tool {
	def := openai.FunctionDefinition{
		Name:        name,
		Description: description,
		Strict:      true,
		Parameters:  parameters,
	}
	return openai.Tool{
		Type:     openai.ToolTypeFunction,
		Function: &def,
	}
}

func animationNames() []string {
	names := make([]string, 0, len(animationMap))
	for _, anim := range animationMap {
		names = append(names, anim[0])
	}
	return names
}

func createAIReqWithToolMode(transcribedText, esn string, gpt3tryagain, isKG, useTools bool) openai.ChatCompletionRequest {
	defaultPrompt := "You are a helpful, animated robot called Vector. Keep the response concise yet informative."

	var nChat []openai.ChatCompletionMessage
	smsg := openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleSystem,
	}
	if strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt) != "" {
		smsg.Content = strings.TrimSpace(vars.APIConfig.Knowledge.OpenAIPrompt)
	} else {
		smsg.Content = defaultPrompt
	}

	var model string
	if gpt3tryagain {
		model = openai.GPT3Dot5Turbo
	} else if vars.APIConfig.Knowledge.Provider == "openai" {
		model = openai.GPT4oMini
		logger.Println("Using " + model)
	} else {
		logger.Println("Using " + vars.APIConfig.Knowledge.Model)
		model = vars.APIConfig.Knowledge.Model
	}

	mode := promptModeNoCommands
	if vars.APIConfig.Knowledge.CommandsEnable {
		if useTools {
			mode = promptModeTools
		} else {
			mode = promptModeLegacyCommands
		}
	}
	smsg.Content = createPromptForMode(smsg.Content, model, isKG, mode)

	nChat = append(nChat, smsg)
	if vars.APIConfig.Knowledge.SaveChat {
		rchat := GetChat(esn)
		logger.Println("Using remembered chats, length of " + fmt.Sprint(len(rchat.Chats)) + " messages")
		nChat = append(nChat, rchat.Chats...)
	}
	nChat = append(nChat, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: transcribedText,
	})

	aireq := openai.ChatCompletionRequest{
		Model:            model,
		Temperature:      1,
		TopP:             1,
		FrequencyPenalty: 0,
		PresencePenalty:  0,
		Messages:         nChat,
		Stream:           true,
	}
	if vars.APIConfig.Knowledge.Provider == "openai" || vars.APIConfig.Knowledge.Provider == "openrouter" {
		aireq.MaxCompletionTokens = 2048
	} else {
		aireq.MaxTokens = 2048
	}
	if vars.APIConfig.Knowledge.CommandsEnable && useTools {
		aireq.Tools = robotToolDefinitions()
		aireq.ToolChoice = "auto"
		aireq.ParallelToolCalls = false
	}
	return aireq
}

func runLLMRuntime(ctx context.Context, client *openai.Client, req openai.ChatCompletionRequest, robot *vector.Vector, stopStop chan bool, ensureReady func(), shouldStop func() bool, emitFirmwareIntent func(string, string) error) (llmRuntimeResult, error) {
	var result llmRuntimeResult
	messages := append([]openai.ChatCompletionMessage(nil), req.Messages...)

	for turn := 0; turn < maxLLMToolTurns; turn++ {
		req.Messages = messages
		turnResult, err := streamLLMTurn(ctx, client, req)
		if err != nil {
			return result, err
		}
		if strings.TrimSpace(turnResult.Content) == "" && len(turnResult.ToolCalls) == 0 {
			logger.Println("LLM empty response decision: no assistant text or tool calls; empty_choice_chunks=" + fmt.Sprint(turnResult.EmptyChoiceChunks) + " reasoning_chars=" + fmt.Sprint(turnResult.ReasoningChars))
			return result, errors.New("llm returned no response")
		}

		if strings.TrimSpace(turnResult.Content) != "" {
			if !result.ReadyStarted {
				result.ReadyStarted = true
				ensureReady()
			}
			if result.FinalText != "" {
				result.FinalText += " "
			}
			result.FinalText += strings.TrimSpace(turnResult.Content)
			for _, segment := range turnResult.Segments {
				if shouldStop() {
					return result, nil
				}
				disconnect := PerformActions(messages, GetActionsFromString(segment), robot, stopStop)
				if disconnect {
					result.Disconnect = true
					return result, nil
				}
			}
		}

		assistantMsg := openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   turnResult.Content,
			ToolCalls: turnResult.ToolCalls,
		}

		if len(turnResult.ToolCalls) == 0 {
			result.Messages = messages
			return result, nil
		}

		messages = append(messages, assistantMsg)
		for _, toolCall := range turnResult.ToolCalls {
			if shouldStop() {
				result.Messages = messages
				return result, nil
			}
			if toolCallRequiresBehaviorReady(toolCall) && !result.ReadyStarted {
				result.ReadyStarted = true
				ensureReady()
			}
			toolResult := executeRobotToolCall(toolCall, robot, stopStop)
			if strings.TrimSpace(toolResult.Output) != "" && toolResult.FirmwareIntent == "" {
				result.ToolActions = append(result.ToolActions, toolCall.Function.Name+": "+toolResult.Output)
			}
			if toolResult.FirmwareIntent != "" {
				if err := deliverFirmwareIntent(toolResult, toolCall.Function.Name, robot, emitFirmwareIntent); err != nil {
					return result, err
				}
			}
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Name:       toolCall.Function.Name,
				ToolCallID: ensureToolCallID(toolCall),
				Content:    toolResult.Output,
			})
			if toolResult.ImageMessage != nil {
				messages = append(messages, *toolResult.ImageMessage)
			}
			if toolResult.Disconnect {
				result.Disconnect = true
				result.Messages = messages
				return result, nil
			}
		}
	}

	result.Messages = messages
	return result, errors.New("llm exceeded tool call loop limit")
}

func toolCallRequiresBehaviorReady(toolCall openai.ToolCall) bool {
	switch toolCall.Function.Name {
	case robotToolPlayAnimation, robotToolPlayAnimationNonInterrupt, robotToolCaptureImage, robotToolRunActionSequence:
		return true
	default:
		return false
	}
}

func deliverFirmwareIntent(toolResult robotToolExecution, toolName string, robot *vector.Vector, emitFirmwareIntent func(string, string) error) error {
	if toolResult.FirmwareIntent == "" {
		return nil
	}
	if emitFirmwareIntent != nil {
		return emitFirmwareIntent(toolResult.FirmwareIntent, toolName)
	}
	return DoRunBuiltinIntent(toolResult.FirmwareIntent, robot)
}

func streamLLMTurn(ctx context.Context, client *openai.Client, req openai.ChatCompletionRequest) (llmTurnResult, error) {
	provider := vars.APIConfig.Knowledge.Provider
	logger.Println("LLM request start provider=" + provider + " model=" + req.Model + " messages=" + fmt.Sprint(len(req.Messages)) + " tools=" + fmt.Sprint(len(req.Tools)))
	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		logger.Println("LLM request error provider=" + provider + " model=" + req.Model + ": " + err.Error())
		return llmTurnResult{}, err
	}
	defer stream.Close()

	acc := newLLMTurnAccumulator(provider, req.Model)
	for {
		raw, err := stream.RecvRaw()
		if errors.Is(err, io.EOF) {
			result := acc.finish()
			logger.Println("LLM stream finished provider=" + provider + " model=" + req.Model + " finish_reason=" + result.FinishReason + " tool_calls=" + fmt.Sprint(len(result.ToolCalls)) + " empty_choice_chunks=" + fmt.Sprint(result.EmptyChoiceChunks) + " final_response_chars=" + fmt.Sprint(len([]rune(result.Content))))
			return result, nil
		}
		if err != nil {
			logger.Println("LLM stream error provider=" + provider + " model=" + req.Model + ": " + err.Error())
			return acc.finish(), err
		}
		if err := acc.addRaw(raw); err != nil {
			logger.Println("LLM stream parse error provider=" + provider + " model=" + req.Model + ": " + err.Error())
			return acc.finish(), err
		}
	}
}

func newLLMTurnAccumulator(provider, model string) *llmTurnAccumulator {
	return &llmTurnAccumulator{
		provider: provider,
		model:    model,
		tools: toolCallAccumulator{
			calls: make(map[int]*toolCallState),
		},
	}
}

func (a *llmTurnAccumulator) addRaw(raw []byte) error {
	a.chunkCount++
	if llmChunkDebugEnabled() {
		logger.Println("LLM chunk debug raw: " + string(raw))
	}

	var chunk llmStreamChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return err
	}
	if chunk.Error != nil {
		logger.Println("LLM provider error code=" + fmt.Sprint(chunk.Error.Code) + " message=" + chunk.Error.Message)
		return errors.New("llm provider error: " + chunk.Error.Message)
	}
	if len(chunk.Choices) == 0 {
		a.emptyChoiceChunks++
		if llmChunkDebugEnabled() {
			logger.Println("LLM chunk debug: empty choices usage_present=" + fmt.Sprint(chunk.Usage != nil))
		}
		return nil
	}

	for _, choice := range chunk.Choices {
		finishReason := choice.FinishReason
		if finishReason == "" {
			finishReason = choice.Delta.FinishReason
		}
		if finishReason != "" {
			a.finishReason = finishReason
		}
		if finishReason == "error" {
			return errors.New("llm provider finished stream with error")
		}

		if choice.Delta.Content != nil {
			a.segments = append(a.segments, a.segmenter.add(removeSpecialCharacters(*choice.Delta.Content))...)
		}
		if choice.Delta.Refusal != nil {
			a.refusalChars += len([]rune(*choice.Delta.Refusal))
			a.segments = append(a.segments, a.segmenter.add(removeSpecialCharacters(*choice.Delta.Refusal))...)
		}
		if choice.Delta.Reasoning != nil {
			a.reasoningChars += len([]rune(*choice.Delta.Reasoning))
		}
		if choice.Delta.ReasoningContent != nil {
			a.reasoningChars += len([]rune(*choice.Delta.ReasoningContent))
		}
		if len(choice.Delta.ReasoningDetails) > 0 {
			a.reasoningChars += len(choice.Delta.ReasoningDetails)
		}
		for _, toolCall := range choice.Delta.ToolCalls {
			a.tools.add(toolCall)
		}
	}

	if llmChunkDebugEnabled() {
		logger.Println("LLM chunk debug parsed choices=" + fmt.Sprint(len(chunk.Choices)) + " content_chars=" + fmt.Sprint(len([]rune(a.segmenter.full.String()))) + " reasoning_chars=" + fmt.Sprint(a.reasoningChars))
	}
	return nil
}

func (a *llmTurnAccumulator) finish() llmTurnResult {
	segments := append([]string(nil), a.segments...)
	segments = append(segments, a.segmenter.finish()...)
	toolCalls := a.tools.finish()
	for _, toolCall := range toolCalls {
		logger.Println("LLM tool call: " + toolCall.Function.Name)
	}
	return llmTurnResult{
		Content:           strings.TrimSpace(a.segmenter.full.String()),
		Segments:          segments,
		ToolCalls:         toolCalls,
		FinishReason:      a.finishReason,
		EmptyChoiceChunks: a.emptyChoiceChunks,
		ReasoningChars:    a.reasoningChars,
		RefusalChars:      a.refusalChars,
	}
}

func llmChunkDebugEnabled() bool {
	return os.Getenv(llmChunkDebugEnv) == "true" || os.Getenv(llmChunkDebugLegacyEnv) == "true"
}

func shouldRetryLLMWithoutTools(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "tool") ||
		strings.Contains(msg, "function") ||
		strings.Contains(msg, "tool_choice") ||
		strings.Contains(msg, "parallel_tool_calls")
}

func (a *toolCallAccumulator) add(toolCall openai.ToolCall) {
	index := len(a.order)
	if toolCall.Index != nil {
		index = *toolCall.Index
	}
	state, ok := a.calls[index]
	if !ok {
		state = &toolCallState{
			Index: index,
			Type:  openai.ToolTypeFunction,
		}
		a.calls[index] = state
		a.order = append(a.order, index)
	}
	if toolCall.ID != "" {
		state.ID = toolCall.ID
	}
	if toolCall.Type != "" {
		state.Type = toolCall.Type
	}
	if toolCall.Function.Name != "" {
		state.Name = toolCall.Function.Name
	}
	if toolCall.Function.Arguments != "" {
		state.Arguments.WriteString(toolCall.Function.Arguments)
	}
}

func (a *toolCallAccumulator) finish() []openai.ToolCall {
	var calls []openai.ToolCall
	for _, index := range a.order {
		state := a.calls[index]
		idx := state.Index
		id := state.ID
		if id == "" {
			id = fmt.Sprintf("call_%s_%d", state.Name, state.Index)
		}
		calls = append(calls, openai.ToolCall{
			Index: &idx,
			ID:    id,
			Type:  state.Type,
			Function: openai.FunctionCall{
				Name:      state.Name,
				Arguments: state.Arguments.String(),
			},
		})
	}
	return calls
}

func (s *sentenceSegmenter) add(delta string) []string {
	if delta == "" {
		return nil
	}
	delta = normalizeLLMDelta(s.full.String(), delta)
	s.pending.WriteString(delta)
	s.full.WriteString(delta)

	var segments []string
	for {
		pending := s.pending.String()
		boundary := sentenceBoundary(pending)
		if boundary == -1 {
			break
		}
		segment := strings.TrimSpace(pending[:boundary])
		if segment != "" {
			segments = append(segments, segment)
		}
		s.pending.Reset()
		s.pending.WriteString(strings.TrimSpace(pending[boundary:]))
	}
	return segments
}

func normalizeLLMDelta(existing, delta string) string {
	if delta == "" || existing == "" {
		return delta
	}
	first := firstRune(delta)
	last := lastRune(existing)
	if unicode.IsSpace(first) || unicode.IsSpace(last) || isNoSpaceBeforeRune(first) || isNoSpaceAfterRune(last) {
		return delta
	}
	if !unicode.IsLetter(last) || !unicode.IsLetter(first) {
		return delta
	}
	prevWord := lastWord(existing)
	nextWord := firstWord(delta)
	pair := strings.ToLower(prevWord + "|" + nextWord)
	if compactBoundaryPairs[pair] {
		return delta
	}
	if unicode.IsUpper(first) || likelyMissingSpaceBetweenWords(prevWord, nextWord) {
		return " " + delta
	}
	return delta
}

func likelyMissingSpaceBetweenWords(prevWord, nextWord string) bool {
	prev := strings.ToLower(prevWord)
	next := strings.ToLower(nextWord)
	if prev == "" || next == "" {
		return false
	}
	return commonBoundaryPrevWords[prev] && commonBoundaryNextWords[next]
}

func firstRune(input string) rune {
	for _, r := range input {
		return r
	}
	return 0
}

func lastRune(input string) rune {
	var last rune
	for _, r := range input {
		last = r
	}
	return last
}

func firstWord(input string) string {
	var b strings.Builder
	for _, r := range input {
		if !unicode.IsLetter(r) {
			break
		}
		b.WriteRune(r)
	}
	return b.String()
}

func lastWord(input string) string {
	runes := []rune(input)
	end := len(runes)
	for end > 0 && !unicode.IsLetter(runes[end-1]) {
		end--
	}
	start := end
	for start > 0 && unicode.IsLetter(runes[start-1]) {
		start--
	}
	return string(runes[start:end])
}

func isNoSpaceBeforeRune(r rune) bool {
	switch r {
	case '.', ',', '?', '!', ':', ';', ')', ']', '}', '\'', '"':
		return true
	default:
		return false
	}
}

func isNoSpaceAfterRune(r rune) bool {
	switch r {
	case '(', '[', '{', '\'', '"':
		return true
	default:
		return false
	}
}

var commonBoundaryPrevWords = map[string]bool{
	"a": true, "about": true, "after": true, "also": true, "am": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "because": true, "but": true, "by": true, "can": true, "could": true, "did": true, "do": true, "does": true,
	"for": true, "from": true, "go": true, "had": true, "has": true, "have": true, "heading": true, "how": true, "i": true,
	"if": true, "in": true, "is": true, "it": true, "just": true, "let": true, "like": true, "may": true, "mean": true,
	"my": true, "no": true, "not": true, "of": true, "ok": true, "okay": true, "on": true, "or": true, "please": true,
	"set": true, "should": true, "so": true, "sure": true, "tell": true, "that": true, "the": true, "then": true, "there": true,
	"to": true, "was": true, "we": true, "what": true, "when": true, "where": true, "which": true, "why": true, "will": true,
	"with": true, "would": true, "yes": true, "you": true, "your": true,
}

var commonBoundaryNextWords = map[string]bool{
	"a": true, "about": true, "after": true, "also": true, "am": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "because": true, "but": true, "by": true, "can": true, "color": true, "could": true, "did": true, "do": true,
	"does": true, "eye": true, "eyes": true, "for": true, "from": true, "go": true, "home": true, "how": true, "i": true,
	"if": true, "in": true, "is": true, "it": true, "like": true, "me": true, "my": true, "now": true, "of": true, "on": true,
	"or": true, "please": true, "set": true, "should": true, "so": true, "that": true, "the": true, "then": true, "thing": true,
	"this": true, "to": true, "up": true, "was": true, "we": true, "what": true, "when": true, "where": true, "which": true,
	"why": true, "will": true, "with": true, "would": true, "you": true, "your": true,
}

var compactBoundaryPairs = map[string]bool{
	"chat|gpt":    true,
	"open|ai":     true,
	"open|router": true,
	"wire|pod":    true,
	"whisper|cpp": true,
}

func (s *sentenceSegmenter) finish() []string {
	var segments []string
	pending := strings.TrimSpace(s.pending.String())
	if pending != "" {
		segments = append(segments, pending)
	}
	s.pending.Reset()
	return segments
}

func sentenceBoundary(input string) int {
	for i := 0; i < len(input); i++ {
		switch input[i] {
		case '.', '?', '!':
			end := i + 1
			for end < len(input) && (input[end] == '"' || input[end] == '\'') {
				end++
			}
			if input[i] == '.' {
				for end < len(input) && input[end] == '.' {
					end++
				}
			}
			return end
		}
	}
	return -1
}

func executeRobotToolCall(toolCall openai.ToolCall, robot *vector.Vector, stopStop chan bool) robotToolExecution {
	name := toolCall.Function.Name
	switch name {
	case robotToolPlayAnimation:
		animation, err := animationArg(toolCall)
		if err != nil {
			return robotToolExecution{Output: err.Error()}
		}
		if err := DoPlayAnimation(animation, robot); err != nil {
			return robotToolExecution{Output: "error: " + err.Error()}
		}
		return robotToolExecution{Output: "ok"}
	case robotToolPlayAnimationNonInterrupt:
		animation, err := animationArg(toolCall)
		if err != nil {
			return robotToolExecution{Output: err.Error()}
		}
		if err := DoPlayAnimationWI(animation, robot); err != nil {
			return robotToolExecution{Output: "error: " + err.Error()}
		}
		return robotToolExecution{Output: "ok"}
	case robotToolRequestNewVoiceInput:
		go DoNewRequest(robot)
		return robotToolExecution{Output: "ok", Disconnect: true}
	case robotToolCaptureImage:
		cameraAngle, err := cameraAngleArg(toolCall)
		if err != nil {
			return robotToolExecution{Output: err.Error()}
		}
		imageMsg, err := CaptureImageMessage(cameraAngle, robot, stopStop)
		if err != nil {
			return robotToolExecution{Output: "error: " + err.Error()}
		}
		return robotToolExecution{
			Output:       "ok: captured image from Vector's " + cameraAngle + " camera view. The image is attached in the next user message.",
			ImageMessage: &imageMsg,
		}
	case robotToolSetEyeColor:
		color, err := eyeColorArg(toolCall)
		if err != nil {
			return robotToolExecution{Output: err.Error()}
		}
		if err := applyCustomEyeColor(color, robot); err != nil {
			return robotToolExecution{Output: "error: " + err.Error()}
		}
		return robotToolExecution{Output: "ok: eye color set to " + color.Hex}
	case robotToolRunActionSequence:
		steps, err := actionSequenceArg(toolCall)
		if err != nil {
			return robotToolExecution{Output: err.Error()}
		}
		if err := DoRunActionSequence(steps, robot); err != nil {
			return robotToolExecution{Output: "error: " + err.Error()}
		}
		return robotToolExecution{Output: "ok: ran " + fmt.Sprint(len(steps)) + " action sequence steps"}
	case robotToolGoToCharger:
		return robotToolExecution{Output: "ok: heading to charger", FirmwareIntent: "intent_system_charger", Disconnect: true}
	case robotToolRunBuiltinIntent:
		intentName, err := robotIntentArg(toolCall)
		if err != nil {
			return robotToolExecution{Output: err.Error()}
		}
		if !validRobotIntent(intentName) {
			return robotToolExecution{Output: "error: invalid built-in robot intent " + intentName}
		}
		return robotToolExecution{Output: "ok: ran built-in intent " + intentName, FirmwareIntent: intentName, Disconnect: true}
	default:
		logger.Println("LLM tried to call unknown tool: " + name)
		return robotToolExecution{Output: "error: unknown tool " + name}
	}
}

func animationArg(toolCall openai.ToolCall) (string, error) {
	var args struct {
		Animation string `json:"animation"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("error: invalid animation arguments: %w", err)
	}
	if !validAnimationName(args.Animation) {
		return "", fmt.Errorf("error: invalid animation %q", args.Animation)
	}
	return args.Animation, nil
}

func cameraAngleArg(toolCall openai.ToolCall) (string, error) {
	var args struct {
		CameraAngle string `json:"camera_angle"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("error: invalid camera arguments: %w", err)
	}
	if args.CameraAngle == "" {
		args.CameraAngle = "front"
	}
	if args.CameraAngle != "front" && args.CameraAngle != "lookingUp" {
		return "", fmt.Errorf("error: invalid camera angle %q", args.CameraAngle)
	}
	return args.CameraAngle, nil
}

func eyeColorArg(toolCall openai.ToolCall) (normalizedEyeColor, error) {
	var args struct {
		HexColor string `json:"hex_color"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return normalizedEyeColor{}, fmt.Errorf("error: invalid eye color arguments: %w", err)
	}
	color, err := normalizeEyeColorHex(args.HexColor)
	if err != nil {
		return normalizedEyeColor{}, fmt.Errorf("error: %w", err)
	}
	return color, nil
}

func robotIntentArg(toolCall openai.ToolCall) (string, error) {
	var args struct {
		Intent string `json:"intent"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return "", fmt.Errorf("error: invalid built-in intent arguments: %w", err)
	}
	if !validRobotIntent(args.Intent) {
		return "", fmt.Errorf("error: invalid built-in robot intent %q", args.Intent)
	}
	return args.Intent, nil
}

type sequenceActionStep struct {
	Action       string  `json:"action"`
	DistanceMM   float32 `json:"distance_mm,omitempty"`
	SpeedMMPS    float32 `json:"speed_mmps,omitempty"`
	AngleDegrees float32 `json:"angle_degrees,omitempty"`
	Animation    string  `json:"animation,omitempty"`
	HexColor     string  `json:"hex_color,omitempty"`
	DurationMS   int     `json:"duration_ms,omitempty"`
}

func actionSequenceArg(toolCall openai.ToolCall) ([]sequenceActionStep, error) {
	var args struct {
		Steps []sequenceActionStep `json:"steps"`
	}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
		return nil, fmt.Errorf("error: invalid action sequence arguments: %w", err)
	}
	if len(args.Steps) == 0 {
		return nil, errors.New("error: action sequence must include at least one step")
	}
	if len(args.Steps) > 8 {
		return nil, errors.New("error: action sequence is limited to 8 steps")
	}
	for i, step := range args.Steps {
		switch step.Action {
		case "drive_straight":
			if step.DistanceMM == 0 {
				return nil, fmt.Errorf("error: step %d drive_straight requires distance_mm", i+1)
			}
		case "turn":
			if step.AngleDegrees == 0 {
				return nil, fmt.Errorf("error: step %d turn requires angle_degrees", i+1)
			}
		case "play_animation", "play_animation_non_interrupting":
			if !validAnimationName(step.Animation) {
				return nil, fmt.Errorf("error: step %d invalid animation %q", i+1, step.Animation)
			}
		case "set_eye_color":
			if _, err := normalizeEyeColorHex(step.HexColor); err != nil {
				return nil, fmt.Errorf("error: step %d invalid eye color: %w", i+1, err)
			}
		case "wait":
			if step.DurationMS <= 0 {
				return nil, fmt.Errorf("error: step %d wait requires duration_ms", i+1)
			}
		default:
			return nil, fmt.Errorf("error: step %d unknown sequence action %q", i+1, step.Action)
		}
	}
	return args.Steps, nil
}

func DoRunActionSequence(steps []sequenceActionStep, robot *vector.Vector) error {
	if robot == nil {
		return errors.New("robot connection is not available")
	}
	for i, step := range steps {
		logger.Println("LLM action sequence step " + fmt.Sprint(i+1) + "/" + fmt.Sprint(len(steps)) + ": " + step.Action)
		switch step.Action {
		case "drive_straight":
			speed := clampFloat32(step.SpeedMMPS, 20, 100)
			if step.SpeedMMPS == 0 {
				speed = 60
			}
			dist := clampFloat32(step.DistanceMM, -300, 300)
			if _, err := robot.Conn.DriveStraight(context.Background(), &vectorpb.DriveStraightRequest{
				SpeedMmps:           speed,
				DistMm:              dist,
				ShouldPlayAnimation: false,
				NumRetries:          1,
			}); err != nil {
				return fmt.Errorf("drive_straight step %d failed: %w", i+1, err)
			}
		case "turn":
			angle := clampFloat32(step.AngleDegrees, -180, 180)
			if _, err := robot.Conn.TurnInPlace(context.Background(), &vectorpb.TurnInPlaceRequest{
				AngleRad:        float32(float64(angle) * math.Pi / 180),
				SpeedRadPerSec:  1.0,
				AccelRadPerSec2: 2.0,
				TolRad:          0,
				IsAbsolute:      0,
				NumRetries:      1,
			}); err != nil {
				return fmt.Errorf("turn step %d failed: %w", i+1, err)
			}
		case "play_animation":
			if err := DoPlayAnimation(step.Animation, robot); err != nil {
				return fmt.Errorf("play_animation step %d failed: %w", i+1, err)
			}
		case "play_animation_non_interrupting":
			if err := DoPlayAnimationWI(step.Animation, robot); err != nil {
				return fmt.Errorf("play_animation_non_interrupting step %d failed: %w", i+1, err)
			}
		case "set_eye_color":
			color, err := normalizeEyeColorHex(step.HexColor)
			if err != nil {
				return fmt.Errorf("set_eye_color step %d failed: %w", i+1, err)
			}
			if err := applyCustomEyeColor(color, robot); err != nil {
				return fmt.Errorf("set_eye_color step %d failed: %w", i+1, err)
			}
		case "wait":
			time.Sleep(time.Duration(clampInt(step.DurationMS, 100, 2000)) * time.Millisecond)
		}
	}
	return nil
}

func clampFloat32(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func validAnimationName(name string) bool {
	for _, anim := range animationMap {
		if anim[0] == name {
			return true
		}
	}
	return false
}

func ensureToolCallID(toolCall openai.ToolCall) string {
	if toolCall.ID != "" {
		return toolCall.ID
	}
	if toolCall.Index != nil {
		return fmt.Sprintf("call_%s_%d", toolCall.Function.Name, *toolCall.Index)
	}
	return "call_" + toolCall.Function.Name
}

func CaptureImageMessage(param string, robot *vector.Vector, stopStop chan bool) (openai.ChatCompletionMessage, error) {
	stopImaging := false
	go func() {
		for range stopStop {
			stopImaging = true
			break
		}
	}()

	logger.Println("Capturing image for LLM...")
	robot.Conn.EnableMirrorMode(context.Background(), &vectorpb.EnableMirrorModeRequest{
		Enable: true,
	})
	defer robot.Conn.EnableMirrorMode(
		context.Background(),
		&vectorpb.EnableMirrorModeRequest{Enable: false},
	)

	for i := 3; i > 0; i-- {
		if stopImaging {
			return openai.ChatCompletionMessage{}, errors.New("image capture interrupted")
		}
		time.Sleep(time.Millisecond * 300)
		robot.Conn.SayText(
			context.Background(),
			&vectorpb.SayTextRequest{
				Text:           fmt.Sprint(i),
				UseVectorVoice: true,
				DurationScalar: 1.05,
			},
		)
	}

	if stopImaging {
		return openai.ChatCompletionMessage{}, errors.New("image capture interrupted")
	}
	resp, err := robot.Conn.CaptureSingleImage(
		context.Background(),
		&vectorpb.CaptureSingleImageRequest{
			EnableHighResolution: true,
		},
	)
	if err != nil {
		return openai.ChatCompletionMessage{}, err
	}

	go func() {
		robot.Conn.PlayAnimation(
			context.Background(),
			&vectorpb.PlayAnimationRequest{
				Animation: &vectorpb.Animation{
					Name: "anim_photo_shutter_01",
				},
				Loops: 1,
			},
		)
	}()

	reqBase64 := base64.StdEncoding.EncodeToString(resp.Data)
	return openai.ChatCompletionMessage{
		Role: openai.ChatMessageRoleUser,
		MultiContent: []openai.ChatMessagePart{
			{
				Type: openai.ChatMessagePartTypeText,
				Text: "Here is the image Vector just captured. Answer the user's visual question using this image.",
			},
			{
				Type: openai.ChatMessagePartTypeImageURL,
				ImageURL: &openai.ChatMessageImageURL{
					URL:    fmt.Sprintf("data:image/jpeg;base64,%s", reqBase64),
					Detail: openai.ImageURLDetailLow,
				},
			},
		},
	}, nil
}
