const els = {};

const state = {
  config: null,
  sdkInfo: null,
  selectedSerial: "",
  battery: null,
  settings: null,
  stats: null,
  activity: null,
  activitySource: null,
  activityStreamSerial: "",
  activityStreamHealthy: false,
  logs: "",
  version: null,
  lastAction: null,
  lastTask: null,
  lastActionAt: 0,
  themeChoice: localStorage.getItem("wirepod-theme") || "system",
};

const languages = [
  ["en-US", "English (US)"],
  ["it-IT", "Italian (IT)"],
  ["es-ES", "Spanish (ES)"],
  ["fr-FR", "French (FR)"],
  ["de-DE", "German (DE)"],
  ["pt-BR", "Portuguese (BR)"],
  ["pl-PL", "Polish (PL)"],
  ["tr-TR", "Turkish (TR)"],
  ["ko-KR", "Korean (KR)"],
  ["zh-CN", "Chinese (CN)"],
  ["ru-RU", "Russian (RU)"],
  ["nt-NL", "Dutch (NL)"],
  ["uk-UA", "Ukrainian (UA)"],
  ["vi-VN", "Vietnamese (VN)"],
];

const whisperModels = [
  ["tiny", "Tiny"],
  ["base", "Base"],
  ["small", "Small"],
  ["medium", "Medium"],
  ["large-v3", "Large v3"],
  ["large-v3-q5_0", "Large v3 q5_0"],
];

const systemIntents = [
  "intent_greeting_hello",
  "intent_names_ask",
  "intent_imperative_eyecolor",
  "intent_character_age",
  "intent_explore_start",
  "intent_system_charger",
  "intent_system_sleep",
  "intent_greeting_goodmorning",
  "intent_greeting_goodnight",
  "intent_greeting_goodbye",
  "intent_imperative_forward",
  "intent_imperative_turnaround",
  "intent_imperative_turnleft",
  "intent_imperative_turnright",
  "intent_play_rollcube",
  "intent_play_popawheelie",
  "intent_play_fistbump",
  "intent_play_blackjack",
  "intent_imperative_affirmative",
  "intent_imperative_negative",
  "intent_photo_take_extend",
  "intent_imperative_praise",
  "intent_imperative_backup",
  "intent_imperative_volumedown",
  "intent_imperative_volumeup",
  "intent_imperative_lookatme",
  "intent_imperative_volumelevel_extend",
  "intent_names_username_extend",
  "intent_imperative_come",
  "intent_imperative_love",
  "intent_knowledge_promptquestion",
  "intent_clock_checktimer",
  "intent_global_stop_extend",
  "intent_clock_settimer_extend",
  "intent_clock_time",
  "intent_imperative_quiet",
  "intent_imperative_dance",
  "intent_play_pickupcube",
  "intent_imperative_fetchcube",
  "intent_imperative_findcube",
  "intent_play_anytrick",
  "intent_message_recordmessage_extend",
  "intent_message_playmessage_extend",
];

const actionMap = {
  wake: { label: "Wake word", task: "Listening for a command", url: "/api-sdk/trigger_wake_word" },
  dock: { label: "Dock robot", task: "Going home", url: "/api-sdk/cloud_intent?intent=intent_system_charger" },
  sleep: { label: "Sleep", task: "Going to sleep", url: "/api-sdk/cloud_intent?intent=intent_system_sleep" },
  dance: { label: "Play animation", task: "Playing an animation", url: "/api-sdk/cloud_intent?intent=intent_imperative_dance" },
  explore: { label: "Explore", task: "Exploring", url: "/api-sdk/cloud_intent?intent=intent_explore_start" },
  "fetch-cube": { label: "Fetch cube", task: "Finding the cube", url: "/api-sdk/cloud_intent?intent=intent_imperative_fetchcube" },
  "go-home": { label: "Go home", task: "Going home", url: "/api-sdk/cloud_intent?intent=intent_system_charger" },
  "take-photo": { label: "Take photo", task: "Taking a photo", url: "/api-sdk/cloud_intent?intent=intent_photo_take_extend" },
  "release-control": { label: "Release behavior control", task: "Releasing behavior control", url: "/api-sdk/release_behavior_control" },
};

const intentTaskLabels = {
  intent_system_charger: "Going home",
  intent_system_sleep: "Going to sleep",
  intent_explore_start: "Exploring",
  intent_imperative_fetchcube: "Finding the cube",
  intent_imperative_findcube: "Finding the cube",
  intent_play_pickupcube: "Picking up the cube",
  intent_play_rollcube: "Rolling the cube",
  intent_photo_take_extend: "Taking a photo",
  intent_imperative_dance: "Dancing",
  intent_play_anytrick: "Doing a trick",
  intent_play_blackjack: "Playing blackjack",
  intent_message_recordmessage_extend: "Recording a message",
  intent_message_playmessage_extend: "Playing a message",
  intent_clock_settimer_extend: "Setting a timer",
  intent_clock_checktimer: "Checking timers",
  intent_clock_time: "Checking the time",
  intent_names_ask: "Answering a name question",
  intent_names_username_extend: "Learning a name",
  intent_knowledge_promptquestion: "Answering a question",
};

const taskWindowMs = 90000;

function init() {
  cacheElements();
  applyTheme();
  populateStaticSelects();
  bindNavigation();
  bindForms();
  bindActions();
  refreshAll();
  setInterval(refreshFast, 3000);
  setInterval(refreshConfig, 15000);
}

function cacheElements() {
  document.querySelectorAll("[id]").forEach((element) => {
    els[toCamel(element.id)] = element;
  });
}

function toCamel(id) {
  return id.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase());
}

function populateStaticSelects() {
  fillSelect(els.sttLanguage, languages);
  fillSelect(els.sttModel, whisperModels);
  fillSelect(els.intentTarget, systemIntents.map((intent) => [intent, intent]));
}

function fillSelect(select, options) {
  if (!select) return;
  select.innerHTML = "";
  options.forEach(([value, label]) => {
    const option = document.createElement("option");
    option.value = value;
    option.textContent = label;
    select.appendChild(option);
  });
}

function bindNavigation() {
  document.querySelectorAll("[data-view]").forEach((button) => {
    button.addEventListener("click", () => showView(button.dataset.view));
  });
  document.querySelectorAll("[data-view-link]").forEach((button) => {
    button.addEventListener("click", () => showView(button.dataset.viewLink));
  });
}

function showView(view) {
  document.querySelectorAll("[data-view]").forEach((button) => {
    const active = button.dataset.view === view;
    button.classList.toggle("is-active", active);
    if (active) {
      button.setAttribute("aria-current", "page");
    } else {
      button.removeAttribute("aria-current");
    }
  });
  document.querySelectorAll("[data-view-panel]").forEach((panel) => {
    panel.classList.toggle("is-active", panel.dataset.viewPanel === view);
  });
}

function bindForms() {
  els.refreshButton.addEventListener("click", refreshAll);
  els.themeToggle.addEventListener("click", cycleTheme);
  els.robotSelect.addEventListener("change", () => {
    state.selectedSerial = els.robotSelect.value;
    persistSelectedRobot();
    updateActivityFeed();
    refreshRobotDetails();
    renderAll();
  });

  document.querySelectorAll("[data-theme-choice]").forEach((button) => {
    button.addEventListener("click", () => {
      state.themeChoice = button.dataset.themeChoice;
      localStorage.setItem("wirepod-theme", state.themeChoice);
      applyTheme();
    });
  });

  els.connectionForm.addEventListener("submit", submitConnection);
  els.speechForm.addEventListener("submit", submitSpeech);
  els.knowledgeForm.addEventListener("submit", submitKnowledge);
  els.weatherForm.addEventListener("submit", submitWeather);
  els.intentForm.addEventListener("submit", submitIntent);
  els.sayTextForm.addEventListener("submit", submitSayText);
  els.kgProvider.addEventListener("change", renderKnowledgeFields);
  els.refreshIntents.addEventListener("click", refreshIntents);
  els.refreshStats.addEventListener("click", refreshStats);
  els.logDebug.addEventListener("change", refreshLogs);
  els.copyLogs.addEventListener("click", copyLogs);
  els.checkUpdates.addEventListener("click", checkUpdates);
}

function bindActions() {
  document.querySelectorAll("[data-action]").forEach((button) => {
    button.addEventListener("click", () => runRobotAction(button.dataset.action));
  });
}

function applyTheme() {
  const resolved = resolveTheme();
  document.documentElement.dataset.theme = resolved;
  document.querySelector("meta[name='theme-color']").setAttribute("content", resolved === "dark" ? "#0f151b" : "#f4f7fa");
  document.querySelectorAll("[data-theme-choice]").forEach((button) => {
    button.classList.toggle("is-active", button.dataset.themeChoice === state.themeChoice);
  });
  els.themeToggle.textContent = resolved === "dark" ? "Light" : "Dark";
}

function resolveTheme() {
  if (state.themeChoice === "dark" || state.themeChoice === "light") {
    return state.themeChoice;
  }
  return window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

function cycleTheme() {
  const resolved = resolveTheme();
  state.themeChoice = resolved === "dark" ? "light" : "dark";
  localStorage.setItem("wirepod-theme", state.themeChoice);
  applyTheme();
}

async function refreshAll() {
  await Promise.allSettled([refreshConfig(), refreshSdkInfo(), refreshLogs(), checkUpdates(false)]);
  await refreshRobotDetails();
  await refreshIntents();
  renderAll();
}

async function refreshFast() {
  await Promise.allSettled([refreshSdkInfo(), refreshLogs()]);
  await refreshRobotDetails();
  renderAll();
}

async function refreshConfig() {
  try {
    state.config = await apiJson("/api/get_config");
    hydrateConfigForms();
  } catch (error) {
    console.warn("Unable to refresh config", error);
  }
}

async function refreshSdkInfo() {
  try {
    state.sdkInfo = await apiJson("/api-sdk/get_sdk_info");
    chooseSelectedRobot();
  } catch (error) {
    state.sdkInfo = null;
    state.selectedSerial = "";
    console.warn("Unable to refresh SDK info", error);
  }
}

async function refreshRobotDetails() {
  if (!state.selectedSerial) {
    state.battery = null;
    state.settings = null;
    state.stats = null;
    state.activity = null;
    closeActivityFeed();
    return;
  }
  updateActivityFeed();
  const refreshes = [refreshBattery(), refreshSettings(), refreshStats()];
  if (!hasLiveActivityFeed()) {
    refreshes.push(refreshActivity());
  }
  await Promise.allSettled(refreshes);
}

async function refreshBattery() {
  try {
    state.battery = await postJson(`/api-sdk/get_battery?serial=${encodeURIComponent(state.selectedSerial)}`);
  } catch (error) {
    state.battery = null;
    console.warn("Unable to refresh battery", error);
  }
}

async function refreshSettings() {
  try {
    state.settings = await postJson(`/api-sdk/get_sdk_settings?serial=${encodeURIComponent(state.selectedSerial)}`);
  } catch (error) {
    state.settings = null;
    console.warn("Unable to refresh robot settings", error);
  }
}

async function refreshStats() {
  if (!state.selectedSerial) return;
  try {
    state.stats = await postJson(`/api-sdk/get_robot_stats?serial=${encodeURIComponent(state.selectedSerial)}`);
    renderStats();
  } catch (error) {
    state.stats = null;
    console.warn("Unable to refresh robot stats", error);
  }
}

async function refreshActivity() {
  if (!state.selectedSerial) return;
  try {
    state.activity = await postJson(`/api-sdk/get_activity?serial=${encodeURIComponent(state.selectedSerial)}`);
  } catch (error) {
    state.activity = null;
    console.warn("Unable to refresh robot activity", error);
  }
}

function updateActivityFeed() {
  if (!("EventSource" in window)) return;
  if (!state.selectedSerial) {
    closeActivityFeed();
    return;
  }
  if (state.activitySource && state.activityStreamSerial === state.selectedSerial && state.activitySource.readyState !== EventSource.CLOSED) {
    return;
  }
  closeActivityFeed();
  state.activityStreamSerial = state.selectedSerial;
  state.activityStreamHealthy = false;
  const source = new EventSource(`/api-sdk/live_activity?serial=${encodeURIComponent(state.selectedSerial)}`);
  state.activitySource = source;
  source.addEventListener("robot_activity", (event) => {
    try {
      state.activity = JSON.parse(event.data);
      state.activityStreamHealthy = true;
      renderAll();
      renderActivity();
    } catch (error) {
      console.warn("Unable to parse robot activity event", error);
    }
  });
  source.onerror = () => {
    state.activityStreamHealthy = false;
  };
}

function closeActivityFeed() {
  if (state.activitySource) {
    state.activitySource.close();
  }
  state.activitySource = null;
  state.activityStreamSerial = "";
  state.activityStreamHealthy = false;
}

function hasLiveActivityFeed() {
  if (!("EventSource" in window)) return false;
  return Boolean(
    state.activitySource &&
    state.activityStreamSerial === state.selectedSerial &&
    state.activitySource.readyState !== EventSource.CLOSED
  );
}

async function refreshLogs() {
  try {
    const endpoint = els.logDebug.checked ? "/api/get_debug_logs" : "/api/get_logs";
    state.logs = await apiText(endpoint);
    renderLogs();
  } catch (error) {
    state.logs = "";
    renderLogs("Unable to load logs.");
  }
}

async function refreshIntents() {
  try {
    const intents = await apiJson("/api/get_custom_intents_json");
    renderIntents(Array.isArray(intents) ? intents : []);
  } catch {
    renderIntents([]);
  }
}

async function checkUpdates(showToast = true) {
  try {
    const versionText = await apiText("/api/get_version_info");
    state.version = JSON.parse(versionText);
    renderVersion();
    if (showToast) toast("Version check complete.", "ok");
  } catch (error) {
    state.version = null;
    renderVersion("Unable to check for updates.");
    if (showToast) toast("Unable to check for updates.", "error");
  }
}

function chooseSelectedRobot() {
  const robots = getRobots();
  const saved = localStorage.getItem("wirepod-selected-robot");
  const hasCurrent = robots.some((robot) => robot.esn === state.selectedSerial);
  if (!hasCurrent) {
    state.selectedSerial = robots.some((robot) => robot.esn === saved) ? saved : (robots[0]?.esn || "");
  }
  updateActivityFeed();
}

function persistSelectedRobot() {
  if (state.selectedSerial) {
    localStorage.setItem("wirepod-selected-robot", state.selectedSerial);
  }
}

function hydrateConfigForms() {
  if (!state.config) return;
  const config = state.config;
  els.connectionMode.value = config.server?.epconfig ? "ep" : "ip";
  els.connectionPort.value = config.server?.port || "443";

  const stt = config.STT || config.stt || {};
  els.sttProvider.value = stt.provider || "vosk";
  els.sttLanguage.value = stt.language || "en-US";
  els.sttModel.value = stt.model || "tiny";
  updateSpeechModelVisibility();

  const knowledge = config.knowledge || {};
  els.kgProvider.value = knowledge.provider || "";
  els.kgIntentgraph.checked = Boolean(knowledge.intentgraph);
  els.kgCommands.checked = Boolean(knowledge.commands_enable);
  els.kgSaveChat.checked = Boolean(knowledge.save_chat);
  renderKnowledgeFields();

  const weather = config.weather || {};
  els.weatherProvider.value = weather.provider || "";
}

function renderAll() {
  renderRobotSelect();
  renderTopbar();
  renderRobotStatus();
  renderServerHealth();
  renderLiveState();
  renderReadiness();
  renderRobotSettings();
  renderStats();
  updateActionAvailability();
  updateDeepLinks();
}

function renderRobotSelect() {
  const robots = getRobots();
  els.robotSelect.innerHTML = "";
  if (robots.length === 0) {
    const option = document.createElement("option");
    option.value = "";
    option.textContent = "No robot";
    els.robotSelect.appendChild(option);
    els.robotSelect.disabled = true;
    return;
  }
  els.robotSelect.disabled = false;
  robots.forEach((robot) => {
    const option = document.createElement("option");
    option.value = robot.esn;
    option.textContent = robot.esn;
    els.robotSelect.appendChild(option);
  });
  els.robotSelect.value = state.selectedSerial;
}

function renderTopbar() {
  const robot = getSelectedRobot();
  const connected = Boolean(robot);
  setDot(els.topRobotDot, connected ? "ok" : "warning");
  els.topRobotStatus.textContent = connected ? "Robot connected" : "No robot connected";
  els.topRobotName.textContent = connected ? `${robot.esn}${robot.ip_address ? ` at ${robot.ip_address}` : ""}` : "Authenticate or connect a robot";
  els.topPipelineState.textContent = deriveState().headline;
}

function renderRobotStatus() {
  const robot = getSelectedRobot();
  const battery = state.battery;
  const batteryPercent = battery ? batteryPercentage(battery.battery_volts) : 0;
  els.robotSerial.textContent = robot?.esn || "Not connected";
  els.robotIp.textContent = robot?.ip_address || "Unavailable";
  els.robotStatusBadge.textContent = robot ? "Connected" : "Missing";
  els.robotStatusBadge.className = `badge ${robot ? "" : "badge-warning"}`;
  els.batteryMeter.setAttribute("aria-valuenow", String(batteryPercent));
  els.batteryMeterFill.style.width = `${batteryPercent}%`;
  els.batteryLabel.textContent = battery ? `${batteryPercent}% (${formatVolts(battery.battery_volts)})` : "Unknown";
  els.robotDock.textContent = battery ? (battery.is_on_charger_platform ? (battery.is_charging ? "Docked, charging" : "Docked") : "Off charger") : "Unknown";
  els.robotFace.src = battery?.is_on_charger_platform ? "assets/expandface.gif" : (robot ? "assets/facegaze.gif" : "assets/wififace.gif");
  els.quickActionContext.textContent = robot ? `Selected ${robot.esn}` : "No robot";
}

function renderServerHealth() {
  const config = state.config;
  const stt = config?.STT || config?.stt;
  const knowledge = config?.knowledge;
  const weather = config?.weather;
  const ready = Boolean(config);
  setDot(els.sidebarServerDot, ready ? "ok" : "warning");
  els.sidebarServerStatus.textContent = ready ? "Server running" : "Server unknown";
  els.apiHealth.textContent = ready ? "OK" : "Unavailable";
  els.setupHealth.textContent = config?.pastinitialsetup ? "Complete" : "Needs setup";
  els.sttHealth.textContent = stt?.provider ? `${stt.provider} / ${stt.language || "unknown"}` : "Not configured";
  els.knowledgeHealth.textContent = knowledge?.enable && knowledge?.provider ? `${knowledge.provider}${knowledge.model ? ` / ${knowledge.model}` : ""}` : "Disabled";
  els.weatherHealth.textContent = weather?.enable && weather?.provider ? weather.provider : "Disabled";
  els.serverHealthBadge.textContent = ready ? "Healthy" : "Unknown";
  els.serverHealthBadge.className = `badge ${ready ? "" : "badge-warning"}`;
  els.connectionBadge.textContent = config?.server?.epconfig ? "Escape Pod" : "IP mode";
  els.speechBadge.textContent = stt?.provider || "Unknown";
  els.knowledgeBadge.textContent = knowledge?.enable && knowledge?.provider ? "Configured" : "Disabled";
  els.knowledgeBadge.className = `badge ${knowledge?.enable && knowledge?.provider ? "" : "badge-muted"}`;
  els.weatherBadge.textContent = weather?.enable && weather?.provider ? "Configured" : "Disabled";
  els.weatherBadge.className = `badge ${weather?.enable && weather?.provider ? "" : "badge-muted"}`;
}

function renderLiveState() {
  const derived = deriveState();
  els.liveStateTitle.textContent = derived.title;
  els.stateSummary.textContent = derived.summary;
  els.stateNow.textContent = derived.now || derived.title || "Unknown";
  els.stateTask.textContent = derived.task || "Nothing active";
  els.liveStateConfidence.textContent = derived.confidence;
  els.liveStateConfidence.className = `badge ${confidenceBadgeClass(derived.confidence)}`;
  els.inspectorStateNote.textContent = derived.detail;
  setPipeline("listen", derived.pipeline.listen);
  setPipeline("stt", derived.pipeline.stt);
  setPipeline("intent", derived.pipeline.intent);
  setPipeline("llm", derived.pipeline.llm);
  setPipeline("action", derived.pipeline.action);
}

function setPipeline(name, value) {
  const element = els[`pipeline${capitalize(name)}`];
  element.querySelector("strong").textContent = value.label;
  element.className = `pipeline-step ${value.kind || ""}`;
}

function deriveState() {
  const robot = getSelectedRobot();
  const recentAction = state.lastAction && Date.now() - state.lastActionAt < 12000;
  const activity = state.activity;
  const latestEntry = latestMeaningfulLogEntry();
  const latestLine = latestEntry.line;
  const freshLog = latestLine && latestEntry.ageMs < 120000;
  const basePipeline = {
    listen: { label: robot ? "Ready" : "No robot", kind: robot ? "ok" : "warning" },
    stt: { label: state.config?.STT?.provider || "Unknown", kind: state.config?.STT?.provider ? "ok" : "warning" },
    intent: { label: "Idle", kind: "" },
    llm: { label: state.config?.knowledge?.enable ? "Ready" : "Off", kind: state.config?.knowledge?.enable ? "ok" : "" },
    action: { label: "Idle", kind: "" },
  };

  if (!robot) {
    return {
      title: "No robot connected",
      headline: "No robot connected",
      now: "No robot",
      task: "Nothing active",
      summary: "Wire-Pod is running, but no authenticated robot is available through the SDK.",
      detail: "Use the setup area or legacy pairing flow to authenticate a robot.",
      confidence: "Confirmed",
      pipeline: basePipeline,
    };
  }

  applyFreshLogPipeline(basePipeline, latestLine, freshLog);

  const activityIsNewerThanAction = activity?.last_event_at && new Date(activity.last_event_at).getTime() >= state.lastActionAt;
  if (activity) {
    const activityState = deriveActivityState(activity, basePipeline);
    if (activityState && (!recentAction || activityIsNewerThanAction || activity.status === "error")) {
      return activityState;
    }
  }

  if (recentAction) {
    basePipeline.action = { label: "Command sent", kind: "active" };
    return {
      title: "Command in progress",
      headline: state.lastAction,
      now: "Command sent",
      task: state.lastTask || state.lastAction,
      summary: `${state.lastAction} was sent to Vector. Waiting for the SDK activity stream to report the next robot state.`,
      detail: "The command request completed in the web console; the robot state will update when the SDK stream emits a new event.",
      confidence: "Sent",
      pipeline: basePipeline,
    };
  }

  if (freshLog) {
    return {
      title: "Recent server activity",
      headline: "Server activity",
      now: "Server activity",
      task: currentTaskLabel() || "Nothing active",
      summary: `${classifyLogLine(latestLine)} log: ${summarizeLogLine(latestLine, 92)}`,
      detail: "This is a fresh Wire-Pod log signal. Robot motion and wake state come from the SDK activity stream when it is available.",
      confidence: "Log signal",
      pipeline: basePipeline,
    };
  }

  const dockState = state.battery?.is_on_charger_platform ? "docked" : "available";
  const historicalNote = latestLine
    ? ` Last visible activity was ${formatAge(latestEntry.ageMs)} ago: ${summarizeLogLine(latestLine, 72)}`
    : "";
  return {
    title: "Idle and ready",
    headline: "Idle",
    now: dockState === "docked" ? "Docked" : "Idle",
    task: "Nothing active",
    summary: `Vector appears ${dockState}. No fresh SDK activity or server log activity is visible.${historicalNote}`,
    detail: "The console listens to Vector's SDK activity stream for wake word, intent, robot-state, and stimulation events.",
    confidence: "Limited",
    pipeline: basePipeline,
  };
}

function deriveActivityState(activity, basePipeline) {
  const pipeline = clonePipeline(basePipeline);
  const ageMs = activityAgeMs(activity);
  const ageLabel = activity.last_event_at ? formatAge(ageMs) : "";
  const robotState = activity.robot_state;
  const wakeWord = activity.wake_word;
  const userIntent = activity.user_intent;
  const recentWake = isActivityRecent(wakeWord?.at, 14000);
  const recentIntent = isActivityRecent(userIntent?.at, 16000);
  const task = currentTaskInfo(activity);
  const robotLabel = robotState?.label || "Idle";
  const robotActive = robotState && (robotState.moving || containsAny(robotLabel.toLowerCase(), ["moving", "animating", "touched", "held", "falling", "detected"]));

  if (activity.status === "error") {
    pipeline.listen = { label: "Stream error", kind: "warning" };
    return {
      title: "Activity stream unavailable",
      headline: "Activity unavailable",
      now: "Activity unavailable",
      task: task?.label || "Nothing active",
      summary: activity.last_error ? `The SDK activity stream returned an error: ${activity.last_error}` : "The SDK activity stream is unavailable.",
      detail: "Robot battery, settings, stats, and logs can still load, but live wake, intent, and motion events are not currently streaming.",
      confidence: "Unavailable",
      pipeline,
    };
  }

  if (activity.status === "connecting" || (activity.stream_active && !activity.last_event_at)) {
    pipeline.listen = { label: "Opening stream", kind: "active" };
    return {
      title: "Opening activity stream",
      headline: "Connecting",
      now: "Connecting",
      task: task?.label || "Nothing active",
      summary: "The console is connected to Vector and waiting for the first SDK activity event.",
      detail: "The live feed listens for wake word, robot state, stimulation, and user intent events from Vector's SDK stream.",
      confidence: "Connecting",
      pipeline,
    };
  }

  if (wakeWord?.state === "listening" && recentWake) {
    pipeline.listen = { label: "Listening", kind: "active" };
    pipeline.stt = { label: "Capturing", kind: "active" };
  }

  if (recentIntent && userIntent) {
    pipeline.intent = { label: compactLabel(task?.label || userIntent.label, 22), kind: "active" };
  }

  if (robotState) {
    pipeline.action = { label: compactLabel(robotLabel, 22), kind: robotActive ? "active" : "ok" };
    if (robotState.being_touched) {
      pipeline.listen = { label: "Touch sensor", kind: "active" };
    }
  }

  if (wakeWord?.state === "complete" && recentWake && !recentIntent) {
    pipeline.listen = { label: wakeWord.intent_heard ? "Intent heard" : "Wake ended", kind: wakeWord.intent_heard ? "ok" : "" };
  }

  const motionDetail = robotState ? robotMotionDetail(robotState) : "";
  if (task && !recentIntent) {
    pipeline.intent = { label: compactLabel(task.label, 22), kind: task.source === "sdk" ? "active" : "" };
  }
  const intentDetail = task ? ` Working on: ${task.label}${task.source === "console" ? " (sent from this console)" : ""}.` : "";
  const wakeDetail = recentWake && wakeWord?.state === "listening" ? " Vector is listening for speech." : "";
  const freshness = ageLabel ? ` Last SDK event was ${ageLabel} ago.` : "";
  const title = recentWake && wakeWord?.state === "listening" ? "Listening" : (recentIntent ? "Intent received" : robotLabel);
  const headline = recentWake && wakeWord?.state === "listening" ? "Listening" : (robotActive ? robotLabel : "Live");

  return {
    title,
    headline,
    now: robotLabel,
    task: task?.label || "Nothing active",
    summary: `Vector is ${robotLabel.toLowerCase()}.${motionDetail}${intentDetail}${wakeDetail}${freshness}`,
    detail: "Live SDK activity stream: wake word, user intent, robot state, and stimulation events.",
    confidence: ageMs <= 15000 ? "Live" : "SDK stream",
    pipeline,
  };
}

function currentTaskInfo(activity = state.activity) {
  const sdkIntent = activity?.user_intent;
  if (sdkIntent?.label && isActivityRecent(sdkIntent.at, taskWindowMs)) {
    return {
      label: normalizeTaskLabel(sdkIntent.label, sdkIntent.json_data),
      source: "sdk",
    };
  }
  if (state.lastTask && Date.now() - state.lastActionAt <= taskWindowMs) {
    return {
      label: state.lastTask,
      source: "console",
    };
  }
  return null;
}

function currentTaskLabel(activity = state.activity) {
  return currentTaskInfo(activity)?.label || "";
}

function normalizeTaskLabel(label, rawJson = "") {
  const text = String(label || "").trim();
  const searchText = `${text} ${rawJson || ""}`.toLowerCase();
  for (const [intent, task] of Object.entries(intentTaskLabels)) {
    if (searchText.includes(intent.toLowerCase())) {
      return task;
    }
  }
  if (text.startsWith("intent_")) {
    return text
      .replace(/^intent_/, "")
      .replace(/_extend$/, "")
      .replace(/_/g, " ")
      .replace(/\b\w/g, (letter) => letter.toUpperCase());
  }
  return text || "Working";
}

function applyFreshLogPipeline(pipeline, line, freshLog) {
  if (!freshLog || !line) return;
  const lower = line.toLowerCase();
  if (containsAny(lower, ["transcrib", "speech", "stt", "heard"])) {
    pipeline.stt = { label: "Recent log", kind: "active" };
  }
  if (containsAny(lower, ["intent", "matched", "utterance"])) {
    pipeline.intent = { label: "Log match", kind: "active" };
  }
  if (containsAny(lower, ["llm", "openai", "openrouter", "knowledge", "prompt", "chat"])) {
    pipeline.llm = { label: "Recent use", kind: "active" };
  }
}

function clonePipeline(pipeline) {
  return Object.fromEntries(Object.entries(pipeline).map(([key, value]) => [key, { ...value }]));
}

function confidenceBadgeClass(confidence) {
  if (confidence === "Live" || confidence === "Confirmed" || confidence === "SDK stream") return "";
  if (confidence === "Unavailable") return "badge-danger";
  if (confidence === "Sent" || confidence === "Log signal" || confidence === "Connecting") return "badge-warning";
  return "badge-muted";
}

function activityAgeMs(activity) {
  if (Number.isFinite(activity?.last_event_age_ms)) {
    return Math.max(0, Number(activity.last_event_age_ms));
  }
  return activity?.last_event_at ? Math.max(0, Date.now() - new Date(activity.last_event_at).getTime()) : Infinity;
}

function isActivityRecent(timestamp, windowMs) {
  return Boolean(timestamp && Date.now() - new Date(timestamp).getTime() <= windowMs);
}

function robotMotionDetail(robotState) {
  const details = [];
  if (robotState.wheel_speed_mmps > 2) details.push(`${Math.round(robotState.wheel_speed_mmps)} mm/s`);
  if (robotState.being_touched) details.push("touch sensor active");
  if (robotState.proximity_detected && robotState.proximity_mm) details.push(`object at ${robotState.proximity_mm} mm`);
  if (details.length === 0) return "";
  return ` ${details.join(", ")}.`;
}

function compactLabel(value, limit) {
  const text = String(value || "").trim();
  return text.length > limit ? `${text.slice(0, limit - 1).trim()}...` : text;
}

function renderReadiness() {
  const checks = [
    ["Server API", Boolean(state.config), state.config ? "Responding" : "No config response"],
    ["Initial setup", Boolean(state.config?.pastinitialsetup), state.config?.pastinitialsetup ? "Complete" : "Needs setup"],
    ["Robot SDK", getRobots().length > 0, getRobots().length > 0 ? `${getRobots().length} robot found` : "No robot found"],
    ["Speech", Boolean(state.config?.STT?.provider), state.config?.STT?.provider || "Not configured"],
    ["Knowledge", Boolean(state.config?.knowledge?.enable && state.config?.knowledge?.provider), state.config?.knowledge?.provider || "Disabled"],
  ];
  els.readinessList.innerHTML = "";
  let readyCount = 0;
  checks.forEach(([label, ok, detail]) => {
    if (ok) readyCount += 1;
    const li = document.createElement("li");
    li.innerHTML = `<span>${escapeHtml(label)}</span><strong>${escapeHtml(detail)}</strong>`;
    els.readinessList.appendChild(li);
  });
  els.readinessBadge.textContent = `${readyCount}/${checks.length} ready`;
  els.readinessBadge.className = `badge ${readyCount === checks.length ? "" : "badge-warning"}`;
  const robot = getSelectedRobot();
  els.inspectorRobot.textContent = robot ? `${robot.esn}${robot.ip_address ? ` at ${robot.ip_address}` : ""}` : "No robot selected yet.";
}

function renderRobotSettings() {
  const settings = state.settings;
  els.robotSettingsStatus.textContent = settings ? "Loaded" : "Unavailable";
  els.robotSettingsStatus.className = `badge ${settings ? "" : "badge-warning"}`;
  els.settingVolume.textContent = settings ? volumeLabel(settings.master_volume) : "Unknown";
  els.settingEyeColor.textContent = settings ? eyeColorLabel(settings) : "Unknown";
  els.settingLocale.textContent = settings?.locale || "Unknown";
  els.settingTime.textContent = settings ? (settings.clock_24_hour ? "24 hour" : "12 hour") : "Unknown";
  els.settingUnits.textContent = settings ? (settings.temp_is_fahrenheit ? "Fahrenheit" : "Celsius") : "Unknown";
  els.settingLocation.textContent = settings?.default_location || "Unknown";
  els.settingTimezone.textContent = settings?.time_zone || "Unknown";
}

function renderStats() {
  if (!els.robotStats) return;
  if (!state.stats) {
    els.robotStats.innerHTML = `<p class="empty">No robot statistics loaded yet.</p>`;
    return;
  }
  const aliveDays = Math.round((state.stats["Alive.seconds"] || 0) / 86400);
  const distance = Math.round((state.stats["Stim.CumlPosDelta"] || 0) / 100);
  const cards = [
    ["Days alive", aliveDays],
    ["Wake word reactions", state.stats["BStat.ReactedToTriggerWord"] || 0],
    ["Utility features used", state.stats["FeatureType.Utility"] || 0],
    ["Seconds petted", Math.round((state.stats["Pet.ms"] || 0) / 1000)],
    ["Distance moved", `${distance} cm`],
  ];
  els.robotStats.innerHTML = cards.map(([label, value]) => `<div class="stat-card"><span>${escapeHtml(label)}</span><strong>${escapeHtml(String(value))}</strong></div>`).join("");
}

function renderLogs(fallback) {
  const text = fallback || state.logs || "No logs yet. Say a command to Vector, then this stream will update.";
  els.logOutput.textContent = text;
  if (els.logAutoscroll.checked) {
    els.logOutput.scrollTop = els.logOutput.scrollHeight;
  }
  renderActivity();
}

function renderActivity() {
  const events = Array.isArray(state.activity?.events) ? state.activity.events.slice(-6).reverse() : [];
  if (events.length > 0) {
    els.activityList.innerHTML = "";
    events.forEach((event) => {
      const row = document.createElement("div");
      row.className = "activity-row";
      row.innerHTML = `<time>${escapeHtml(formatActivityTime(event.at))}</time><strong>${escapeHtml(event.label || "Activity")}</strong><span>${escapeHtml(event.detail || event.type || "")}</span>`;
      els.activityList.appendChild(row);
    });
    return;
  }

  if (state.activity?.status === "connecting") {
    els.activityList.innerHTML = `<p class="empty">Waiting for the first SDK activity event...</p>`;
    return;
  }

  if (state.activity?.status === "error") {
    els.activityList.innerHTML = `<p class="empty">SDK activity stream error: ${escapeHtml(state.activity.last_error || "unknown error")}</p>`;
    return;
  }

  const lines = meaningfulLogLines().slice(-4).reverse();
  if (lines.length === 0) {
    els.activityList.innerHTML = `<p class="empty">No SDK activity events yet.</p>`;
    return;
  }

  els.activityList.innerHTML = "";
  lines.forEach((line) => {
    const row = document.createElement("div");
    row.className = "activity-row";
    row.innerHTML = `<time>${escapeHtml(formatLogLineTime(line))}</time><strong>Server log</strong><span>${escapeHtml(`${classifyLogLine(line)}: ${summarizeLogLine(line)}`)}</span>`;
    els.activityList.appendChild(row);
  });
}

function renderIntents(intents) {
  if (!intents || intents.length === 0) {
    els.intentList.innerHTML = `<p class="empty">No custom intents found. Add one to create a custom utterance route.</p>`;
    return;
  }
  els.intentList.innerHTML = "";
  intents
    .filter((intent) => !intent.issystem)
    .forEach((intent, index) => {
      const row = document.createElement("div");
      row.className = "intent-row";
      row.innerHTML = `<div><strong>${escapeHtml(intent.name || "Unnamed intent")}</strong><span>${escapeHtml((intent.utterances || []).join(", ") || "No utterances")}</span></div>`;
      const button = document.createElement("button");
      button.className = "button button-danger";
      button.type = "button";
      button.textContent = "Delete";
      button.addEventListener("click", () => deleteIntent(index + 1));
      row.appendChild(button);
      els.intentList.appendChild(row);
    });
}

function renderVersion(error) {
  if (error) {
    els.versionDetails.innerHTML = `<p class="empty">${escapeHtml(error)}</p>`;
    return;
  }
  if (!state.version) {
    els.versionDetails.innerHTML = `<p class="empty">No version check has run yet.</p>`;
    return;
  }
  const version = state.version;
  const cards = [
    ["Installed", version.fromsource ? version.installedcommit : version.installedversion],
    ["Current", version.fromsource ? version.currentcommit : version.currentversion],
    ["Update", version.avail ? "Available" : "Up to date"],
  ];
  els.sidebarVersion.textContent = version.fromsource ? `Commit ${version.installedcommit}` : `v${version.installedversion}`;
  els.versionDetails.innerHTML = cards.map(([label, value]) => `<div class="version-card"><span>${escapeHtml(label)}</span><strong>${escapeHtml(String(value || "Unknown"))}</strong></div>`).join("");
}

function renderKnowledgeFields() {
  const provider = els.kgProvider.value;
  const knowledge = state.config?.knowledge || {};
  const field = (id, label, value = "", type = "text", placeholder = "") => `
    <label for="${id}">${label}</label>
    <input id="${id}" type="${type}" value="${escapeAttribute(value || "")}" placeholder="${escapeAttribute(placeholder)}" autocomplete="off">
  `;
  const promptField = (id, label, value = "") => `
    <label for="${id}">${label}</label>
    <textarea id="${id}" rows="4">${escapeHtml(value || "")}</textarea>
  `;
  let html = "";
  if (provider === "openai") {
    html += field("kg-key", "OpenAI key", "", "password", knowledge.key ? "Configured. Leave blank to keep existing key." : "sk-...");
    html += promptField("kg-prompt", "Robot prompt", knowledge.openai_prompt);
    html += `<label for="kg-voice">TTS voice</label><select id="kg-voice">${["fable", "alloy", "echo", "onyx", "nova", "shimmer"].map((voice) => `<option value="${voice}" ${knowledge.openai_voice === voice ? "selected" : ""}>${capitalize(voice)}</option>`).join("")}</select>`;
    html += `<label class="switch-row"><input id="kg-voice-english" type="checkbox" ${knowledge.openai_voice_with_english ? "checked" : ""}><span>Use OpenAI voice for English</span></label>`;
  } else if (provider === "openrouter") {
    html += field("kg-key", "OpenRouter key", "", "password", knowledge.key ? "Configured. Leave blank to keep existing key." : "sk-or-...");
    html += field("kg-model", "Model", knowledge.model || "", "text", "openai/gpt-oss-120b:nitro");
    html += promptField("kg-prompt", "Robot prompt", knowledge.openai_prompt);
  } else if (provider === "custom") {
    html += field("kg-key", "API key", "", "password", knowledge.key ? "Configured. Leave blank to keep existing key." : "For Ollama this can be ollama");
    html += field("kg-endpoint", "Endpoint", knowledge.endpoint || "", "text", "http://localhost:11434/v1");
    html += field("kg-model", "Model", knowledge.model || "", "text", "llama3");
    html += promptField("kg-prompt", "Robot prompt", knowledge.openai_prompt);
  } else if (provider === "together") {
    html += field("kg-key", "Together key", "", "password", knowledge.key ? "Configured. Leave blank to keep existing key." : "");
    html += field("kg-model", "Model", knowledge.model || "", "text", "meta-llama/Llama-3-70b-chat-hf");
    html += promptField("kg-prompt", "Robot prompt", knowledge.openai_prompt);
  } else if (provider === "houndify") {
    html += field("kg-id", "Houndify client ID", knowledge.id || "");
    html += field("kg-key", "Houndify key", "", "password", knowledge.key ? "Configured. Leave blank to keep existing key." : "");
  }
  els.kgProviderFields.innerHTML = html;
}

function updateSpeechModelVisibility() {
  els.sttModel.disabled = els.sttProvider.value !== "whisper.cpp";
}

async function submitConnection(event) {
  event.preventDefault();
  const mode = els.connectionMode.value;
  const port = els.connectionPort.value.trim() || "443";
  setStatus(els.connectionStatus, "Applying connection mode...");
  try {
    const response = mode === "ep"
      ? await apiText("/api-chipper/use_ep")
      : await apiText(`/api-chipper/use_ip?port=${encodeURIComponent(port)}`);
    setStatus(els.connectionStatus, response || "Connection mode applied.", "ok");
    await refreshConfig();
    renderAll();
  } catch (error) {
    setStatus(els.connectionStatus, "Unable to apply connection mode.", "error");
  }
}

async function submitSpeech(event) {
  event.preventDefault();
  const data = {
    provider: els.sttProvider.value,
    language: els.sttLanguage.value,
    model: els.sttModel.value || "tiny",
  };
  setStatus(els.speechStatus, "Saving speech settings...");
  try {
    const response = await postText("/api/set_stt_info", JSON.stringify(data), "application/json");
    setStatus(els.speechStatus, response, "ok");
    if (response.includes("downloading")) {
      pollDownloadStatus();
    }
    await refreshConfig();
    renderAll();
  } catch (error) {
    setStatus(els.speechStatus, "Unable to save speech settings.", "error");
  }
}

function pollDownloadStatus() {
  const interval = setInterval(async () => {
    try {
      const response = await apiText("/api/get_download_status");
      setStatus(els.speechStatus, response.includes("not downloading") ? "Initiating download..." : response);
      if (response.includes("success") || response.includes("error")) {
        clearInterval(interval);
        setStatus(els.speechStatus, response, response.includes("success") ? "ok" : "error");
      }
    } catch {
      clearInterval(interval);
      setStatus(els.speechStatus, "Unable to read download status.", "error");
    }
  }, 700);
}

async function submitKnowledge(event) {
  event.preventDefault();
  const provider = els.kgProvider.value;
  const existing = state.config?.knowledge || {};
  const data = {
    enable: Boolean(provider),
    provider,
    key: valueOrExisting("kg-key", existing.key),
    model: getFieldValue("kg-model"),
    id: getFieldValue("kg-id"),
    intentgraph: els.kgIntentgraph.checked,
    robotName: existing.robotName || "",
    openai_prompt: getFieldValue("kg-prompt"),
    openai_voice: getFieldValue("kg-voice"),
    openai_voice_with_english: getFieldChecked("kg-voice-english"),
    save_chat: els.kgSaveChat.checked,
    commands_enable: els.kgCommands.checked,
    endpoint: getFieldValue("kg-endpoint"),
    top_p: existing.top_p || 0,
    temp: existing.temp || 0,
  };
  if (!provider) {
    data.key = "";
    data.enable = false;
  }
  setStatus(els.knowledgeStatus, "Saving knowledge settings...");
  try {
    const response = await postText("/api/set_kg_api", JSON.stringify(data), "application/json");
    setStatus(els.knowledgeStatus, response, "ok");
    await refreshConfig();
    renderAll();
  } catch (error) {
    setStatus(els.knowledgeStatus, "Unable to save knowledge settings.", "error");
  }
}

async function submitWeather(event) {
  event.preventDefault();
  const existing = state.config?.weather || {};
  const data = {
    provider: els.weatherProvider.value,
    key: els.weatherKey.value.trim() || existing.key || "",
  };
  if (!data.provider) data.key = "";
  setStatus(els.weatherStatus, "Saving weather settings...");
  try {
    const response = await postText("/api/set_weather_api", JSON.stringify(data), "application/json");
    els.weatherKey.value = "";
    setStatus(els.weatherStatus, response, "ok");
    await refreshConfig();
    renderAll();
  } catch {
    setStatus(els.weatherStatus, "Unable to save weather settings.", "error");
  }
}

async function submitIntent(event) {
  event.preventDefault();
  const data = {
    name: els.intentName.value.trim(),
    description: els.intentDescription.value.trim(),
    utterances: splitList(els.intentUtterances.value),
    intent: els.intentTarget.value,
    params: { paramname: "", paramvalue: "" },
    exec: els.intentExec.value.trim(),
    execargs: [],
    luascript: els.intentLua.value.trim(),
  };
  if (!data.name || !data.description || data.utterances.length === 0 || !data.intent) {
    setStatus(els.intentStatus, "Name, description, utterances, and target intent are required.", "error");
    return;
  }
  setStatus(els.intentStatus, "Adding custom intent...");
  try {
    const response = await postText("/api/add_custom_intent", JSON.stringify(data), "application/json");
    setStatus(els.intentStatus, response, "ok");
    els.intentForm.reset();
    await refreshIntents();
  } catch (error) {
    setStatus(els.intentStatus, "Unable to add custom intent.", "error");
  }
}

async function deleteIntent(number) {
  if (!confirm("Delete this custom intent?")) return;
  try {
    await postText("/api/remove_custom_intent", JSON.stringify({ number }), "application/json");
    toast("Intent deleted.", "ok");
    await refreshIntents();
  } catch {
    toast("Unable to delete intent.", "error");
  }
}

async function submitSayText(event) {
  event.preventDefault();
  const text = els.sayText.value.trim();
  if (!text) return;
  await runRobotUrl(`/api-sdk/say_text?text=${encodeURIComponent(text)}`, "Say text", "Speaking text");
  els.sayText.value = "";
}

async function runRobotAction(action) {
  const actionConfig = actionMap[action];
  if (!actionConfig) return;
  if (action === "release-control" && !confirm("Release behavior control for this robot?")) return;
  await runRobotUrl(actionConfig.url, actionConfig.label, actionConfig.task);
}

async function runRobotUrl(url, label, task = label) {
  if (!state.selectedSerial) {
    toast("Select a connected robot first.", "error");
    return;
  }
  try {
    const separator = url.includes("?") ? "&" : "?";
    await postText(`${url}${separator}serial=${encodeURIComponent(state.selectedSerial)}`);
    state.lastAction = label;
    state.lastTask = task;
    state.lastActionAt = Date.now();
    toast(`${label} sent.`, "ok");
    renderLiveState();
  } catch (error) {
    toast(`Unable to send ${label}.`, "error");
  }
}

function updateActionAvailability() {
  const disabled = !state.selectedSerial;
  document.querySelectorAll("[data-action]").forEach((button) => {
    button.disabled = disabled;
  });
  els.sayText.disabled = disabled;
  els.sayTextForm.querySelector("button").disabled = disabled;
}

function updateDeepLinks() {
  const serial = state.selectedSerial ? `?serial=${encodeURIComponent(state.selectedSerial)}` : "";
  els.advancedRobotLink.href = state.selectedSerial ? `/sdkapp/settings.html${serial}` : "/sdkapp";
  els.robotSettingsLink.href = els.advancedRobotLink.href;
  els.robotControlLink.href = state.selectedSerial ? `/sdkapp/control.html${serial}` : "/sdkapp";
}

function valueOrExisting(id, existing) {
  const value = getFieldValue(id);
  return value || existing || "";
}

function getFieldValue(id) {
  const field = document.getElementById(id);
  return field ? field.value.trim() : "";
}

function getFieldChecked(id) {
  const field = document.getElementById(id);
  return field ? field.checked : false;
}

function splitList(value) {
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

async function apiJson(url) {
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) throw new Error(`${url} returned ${response.status}`);
  return response.json();
}

async function apiText(url) {
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) throw new Error(`${url} returned ${response.status}`);
  return response.text();
}

async function postJson(url) {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`${url} returned ${response.status}`);
  return response.json();
}

async function postText(url, body, contentType) {
  const response = await fetch(url, {
    method: "POST",
    headers: contentType ? { "Content-Type": contentType } : { "Content-Type": "application/x-www-form-urlencoded" },
    body,
    cache: "no-store",
  });
  if (!response.ok) throw new Error(`${url} returned ${response.status}`);
  return response.text();
}

function getRobots() {
  return Array.isArray(state.sdkInfo?.robots) ? state.sdkInfo.robots : [];
}

function getSelectedRobot() {
  return getRobots().find((robot) => robot.esn === state.selectedSerial);
}

function batteryPercentage(voltage) {
  if (!voltage) return 70;
  const maxVoltage = 4.1;
  const midVoltage = 3.85;
  const minVoltage = 3.5;
  let percentage;
  if (voltage >= maxVoltage) {
    percentage = 100;
  } else if (voltage >= midVoltage) {
    const scaled = (voltage - midVoltage) / (maxVoltage - midVoltage);
    percentage = 80 + 20 * Math.log10(1 + scaled * 9);
  } else if (voltage >= minVoltage) {
    const scaled = (voltage - minVoltage) / (midVoltage - minVoltage);
    percentage = 80 * Math.log10(1 + scaled * 9);
  } else {
    percentage = 0;
  }
  return Math.max(0, Math.min(100, Math.round(percentage)));
}

function formatVolts(voltage) {
  return voltage ? `${voltage.toFixed(2)}V` : "voltage unknown";
}

function volumeLabel(value) {
  return ["Mute", "Low", "Medium low", "Medium", "Medium high", "High"][Number(value)] || "Unknown";
}

function eyeColorLabel(settings) {
  if (settings.custom_eye_color?.enabled) return "Custom";
  return ["Teal", "Orange", "Yellow", "Lime green", "Azure blue", "Purple", "Other green"][Number(settings.eye_color)] || "Unknown";
}

function meaningfulLogLines() {
  return (state.logs || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
    .filter((line) => !line.includes("/api/get_logs"));
}

function latestMeaningfulLogLine() {
  const lines = meaningfulLogLines();
  return lines[lines.length - 1] || "";
}

function latestMeaningfulLogEntry() {
  const line = latestMeaningfulLogLine();
  return {
    line,
    ageMs: logLineAgeMs(line),
  };
}

function logLineAgeMs(line) {
  const match = line.match(/^(\d{4})\.(\d{2})\.(\d{2})\s+(\d{2}):(\d{2}):(\d{2}):/);
  if (!match) return 0;
  const [, year, month, day, hour, minute, second] = match.map(Number);
  const timestamp = Date.UTC(year, month - 1, day, hour, minute, second);
  return Math.max(0, Date.now() - timestamp);
}

function classifyLogLine(line) {
  const lower = line.toLowerCase();
  if (containsAny(lower, ["error", "failed", "fatal"])) return "Error";
  if (containsAny(lower, ["warning", "warn"])) return "Warning";
  if (containsAny(lower, ["intent", "matched"])) return "Intent";
  if (containsAny(lower, ["llm", "openai", "openrouter", "knowledge"])) return "LLM";
  if (containsAny(lower, ["transcrib", "speech", "heard", "stt"])) return "Speech";
  if (containsAny(lower, ["battery", "charger", "dock"])) return "Robot";
  return "System";
}

function summarizeLogLine(line, limit = 180) {
  const cleaned = line
    .replace(/^\d{4}\.\d{2}\.\d{2}\s+\d{2}:\d{2}:\d{2}:\s*/, "")
    .replace(/\b(Could|Would|Should|Can|Do|Did|Tell|Let|Please)(you|me|us|it|them)\b/gi, "$1 $2")
    .replace(/([.!?])([A-Z])/g, "$1 $2")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/\s+/g, " ")
    .trim();
  return cleaned.length > limit ? `${cleaned.slice(0, limit - 1).trim()}...` : cleaned;
}

function formatAge(ageMs) {
  if (ageMs < 10000) return "just now";
  if (ageMs < 60000) return `${Math.round(ageMs / 1000)} seconds`;
  if (ageMs < 3600000) return `${Math.round(ageMs / 60000)} minutes`;
  return `${Math.round(ageMs / 3600000)} hours`;
}

function formatActivityTime(timestamp) {
  const date = timestamp ? new Date(timestamp) : new Date();
  if (Number.isNaN(date.getTime())) return "Now";
  return date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" });
}

function formatLogLineTime(line) {
  const match = line.match(/^(\d{4})\.(\d{2})\.(\d{2})\s+(\d{2}):(\d{2}):(\d{2}):/);
  if (!match) return "Log";
  return `${match[4]}:${match[5]}:${match[6]}`;
}

function containsAny(text, needles) {
  return needles.some((needle) => text.includes(needle));
}

function setDot(element, status) {
  element.className = `status-dot ${status || ""}`;
}

function setStatus(element, message, kind = "") {
  element.textContent = message;
  element.className = `form-status ${kind}`;
}

function toast(message, kind = "") {
  const node = document.createElement("div");
  node.className = `toast ${kind}`;
  node.textContent = message;
  els.toastRegion.appendChild(node);
  setTimeout(() => {
    node.remove();
  }, 4200);
}

async function copyLogs() {
  try {
    await navigator.clipboard.writeText(els.logOutput.textContent);
    toast("Logs copied.", "ok");
  } catch {
    toast("Unable to copy logs.", "error");
  }
}

function capitalize(value) {
  if (!value) return "";
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function escapeHtml(value) {
  return String(value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

function escapeAttribute(value) {
  return escapeHtml(value).replace(/`/g, "&#096;");
}

window.matchMedia("(prefers-color-scheme: light)").addEventListener("change", () => {
  if (state.themeChoice === "system") applyTheme();
});

document.addEventListener("DOMContentLoaded", init);
