import { useEffect, useState, type CSSProperties } from "react";

import {
  ApplyKeySettings,
  ClearKeyAction,
  GetDashboardState,
  SaveToDevice,
} from "../bindings/hpad-app/agentservice.js";
import { DashboardState } from "../bindings/hpad-app/internal/app/models.js";
import { KeyAction, KeyAssignment } from "../bindings/hpad-app/internal/config/models.js";

const ACTION_UNASSIGNED = "unassigned";
const ACTION_KEYBOARD_SHORTCUT = "keyboard_shortcut";
const ACTION_RUN_COMMAND = "run_command";
const ACTION_OPEN_APPLICATION = "open_application";
const ACTION_MEDIA_CONTROL = "media_control";
const DEFAULT_KEY_COLOR = "#000000";
const DEFAULT_KEY_BRIGHTNESS = 0xff;
const BRIGHTNESS_MAX = 0xff;

const STATUS_REFRESH_MS = 2500;

const actionOptions = [
  { value: ACTION_UNASSIGNED, label: "Unassigned" },
  { value: ACTION_KEYBOARD_SHORTCUT, label: "Keyboard shortcut" },
  { value: ACTION_RUN_COMMAND, label: "Run command" },
  { value: ACTION_OPEN_APPLICATION, label: "Open application" },
  { value: ACTION_MEDIA_CONTROL, label: "Media control" },
];

const mediaOptions = [
  { value: "play_pause", label: "Play / Pause" },
  { value: "next_track", label: "Next track" },
  { value: "previous_track", label: "Previous track" },
  { value: "volume_up", label: "Volume up" },
  { value: "volume_down", label: "Volume down" },
  { value: "mute", label: "Mute" },
];

export default function App() {
  const [dashboard, setDashboard] = useState<DashboardState>(() => createEmptyDashboardState());
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [editorOpen, setEditorOpen] = useState(false);
  const [draftAction, setDraftAction] = useState<KeyAction>(() => createEmptyAction());
  const [draftColor, setDraftColor] = useState(DEFAULT_KEY_COLOR);
  const [draftColorInput, setDraftColorInput] = useState(DEFAULT_KEY_COLOR);
  const [draftBrightness, setDraftBrightness] = useState(DEFAULT_KEY_BRIGHTNESS);
  const [draftDirty, setDraftDirty] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const hydrateDraft = (assignment?: KeyAssignment) => {
    const nextColor = getAssignmentColor(assignment);
    const nextBrightness = getAssignmentBrightness(assignment);
    setDraftAction(copyAction(assignment?.action));
    setDraftColor(nextColor);
    setDraftColorInput(nextColor);
    setDraftBrightness(nextBrightness);
    setDraftDirty(false);
  };

  useEffect(() => {
    let active = true;

    const refresh = async (syncDraft: boolean) => {
      try {
        const next = await GetDashboardState();
        if (!active) {
          return;
        }
        setDashboard(next);
        setErrorMessage(null);
        if (syncDraft) {
          hydrateDraft(next.keyAssignments[selectedIndex]);
        }
      } catch (error) {
        if (!active) {
          return;
        }
        setErrorMessage(getErrorMessage(error));
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    };

    void refresh(!draftDirty);
    const timer = window.setInterval(() => {
      void refresh(!draftDirty);
    }, STATUS_REFRESH_MS);

    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [draftDirty, selectedIndex]);

  const selectedAssignment = dashboard.keyAssignments[selectedIndex] ?? createDefaultAssignment(selectedIndex);
  const configStateLabel = dashboard.dirty ? "Unsaved Changes" : "Saved";
  const saveButtonLabel = busy ? "Saving..." : "Save to Device";

  const handleSelectKey = (index: number) => {
    setSelectedIndex(index);
    hydrateDraft(dashboard.keyAssignments[index]);
    setEditorOpen(true);
    setErrorMessage(null);
  };

  const handleCloseEditor = () => {
    if (busy) {
      return;
    }
    hydrateDraft(dashboard.keyAssignments[selectedIndex]);
    setEditorOpen(false);
  };

  const handleActionTypeChange = (nextType: string) => {
    setDraftAction((current: KeyAction) => normalizeAction({ ...current, type: nextType }));
    setDraftDirty(true);
  };

  const handleDraftFieldChange = (field: keyof KeyAction, value: string) => {
    setDraftAction((current: KeyAction) => ({ ...current, [field]: value }));
    setDraftDirty(true);
  };

  const handleColorPickerChange = (value: string) => {
    const nextColor = normalizeColor(value);
    setDraftColor(nextColor);
    setDraftColorInput(nextColor);
    setDraftDirty(true);
  };

  const handleColorInputChange = (value: string) => {
    const nextInput = sanitizeColorInput(value);
    setDraftColorInput(nextInput);
    if (isValidColor(nextInput)) {
      setDraftColor(nextInput);
    }
    setDraftDirty(true);
  };

  const handleColorInputBlur = () => {
    const nextColor = normalizeColor(draftColorInput || draftColor);
    setDraftColor(nextColor);
    setDraftColorInput(nextColor);
  };

  const handleResetColor = () => {
    setDraftColor(DEFAULT_KEY_COLOR);
    setDraftColorInput(DEFAULT_KEY_COLOR);
    setDraftDirty(true);
  };

  const handleBrightnessChange = (value: number) => {
    setDraftBrightness(normalizeBrightness(value));
    setDraftDirty(true);
  };

  const handleApply = async () => {
    setBusy(true);
    try {
      const next = await ApplyKeySettings(
        selectedIndex,
        normalizeAction(draftAction),
        normalizeColor(draftColorInput),
        normalizeBrightness(draftBrightness),
      );
      setDashboard(next);
      hydrateDraft(next.keyAssignments[selectedIndex]);
      setEditorOpen(false);
      setErrorMessage(null);
    } catch (error) {
      setErrorMessage(getErrorMessage(error));
    } finally {
      setBusy(false);
    }
  };

  const handleClear = async () => {
    setBusy(true);
    try {
      const next = await ClearKeyAction(selectedIndex);
      setDashboard(next);
      hydrateDraft(next.keyAssignments[selectedIndex]);
      setErrorMessage(null);
    } catch (error) {
      setErrorMessage(getErrorMessage(error));
    } finally {
      setBusy(false);
    }
  };

  const handleSave = async () => {
    setBusy(true);
    try {
      const next = await SaveToDevice();
      setDashboard(next);
      setErrorMessage(null);
    } catch (error) {
      setErrorMessage(getErrorMessage(error));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="app-shell">
      <div className="backdrop-grid" aria-hidden="true" />
      <header className="topbar panel">
        <div className="brand-block">
          <p className="eyebrow">HPAD</p>
          <h1>HPAD</h1>
          <p className="subtitle">Compact desktop control panel for a six-key macropad.</p>
        </div>
        <div className="header-statuses">
          <StatusChip title="Dongle" status={dashboard.dongleStatus.label} tone={dashboard.dongleStatus.state} />
          <StatusChip title="Macropad" status={dashboard.macropadStatus.label} tone={dashboard.macropadStatus.state} />
          <StatusChip title="Battery" status={getBatteryChipStatus(dashboard.batteryStatus)} tone={dashboard.batteryStatus.state} />
        </div>
      </header>

      {errorMessage ? <div className="error-banner">{errorMessage}</div> : null}

      <div className="workspace">
        <div className="main-column">
          <section className="panel section-panel">
            <div className="section-header">
              <div>
                <p className="section-kicker">Overview</p>
                <h2>Device Status</h2>
              </div>
              <div className="profile-pill">{dashboard.profile}</div>
            </div>
            <div className="status-grid">
              <StatusCard
                title="Dongle"
                status={dashboard.dongleStatus.label}
                detail={dashboard.dongleStatus.path || dashboard.dongleStatus.detail}
                tone={dashboard.dongleStatus.state}
              />
              <StatusCard
                title="Macropad"
                status={dashboard.macropadStatus.label}
                detail={dashboard.macropadStatus.detail}
                tone={dashboard.macropadStatus.state}
              />
              <StatusCard
                title="Battery"
                status={dashboard.batteryStatus.label}
                detail={dashboard.batteryStatus.detail}
                tone={dashboard.batteryStatus.state}
              />
            </div>
          </section>

          <section className="panel section-panel">
            <div className="section-header">
              <div>
                <p className="section-kicker">Layout</p>
                <h2>Key Assignments</h2>
              </div>
              <p className="section-copy">Click a key to open its action and LED color menu.</p>
            </div>
            <div className="key-grid">
              {dashboard.keyAssignments.map((assignment: KeyAssignment, index: number) => (
                <button
                  key={assignment.id || index}
                  type="button"
                  className={`key-card ${editorOpen && index === selectedIndex ? "selected" : ""} ${isAssigned(assignment) ? "assigned" : "empty"}`}
                  onClick={() => handleSelectKey(index)}
                  style={getColorStyle(getAssignmentColor(assignment))}
                >
                  <div className="key-card-header">
                    <span className="key-label">{assignment.label}</span>
                    <span className="key-state">{isAssigned(assignment) ? "Assigned" : "Open"}</span>
                  </div>
                  <div className="key-led-row">
                    <span className="key-led-preview" aria-hidden="true" />
                    <span className="key-led-value">{getLEDSummary(assignment)}</span>
                  </div>
                  <p className="key-summary">{getActionSummary(assignment.action)}</p>
                </button>
              ))}
            </div>
          </section>
        </div>
      </div>

      <footer className="footerbar panel">
        <div className="footer-state-block">
          <span className={`footer-state ${dashboard.dirty ? "dirty" : "saved"}`}>{configStateLabel}</span>
          <span className="footer-copy">
            {dashboard.dirty ? "Changes are staged in the layout and not yet saved to the device." : "Configuration is in sync with the saved layout."}
          </span>
        </div>
        <button type="button" className="button button-primary footer-button" onClick={handleSave} disabled={busy || !dashboard.dirty}>
          {saveButtonLabel}
        </button>
      </footer>

      {editorOpen ? (
        <div className="key-editor-backdrop" onClick={handleCloseEditor}>
          <section
            className="panel key-editor-dialog"
            role="dialog"
            aria-modal="true"
            aria-labelledby="key-editor-title"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="key-editor-header">
              <div>
                <p className="section-kicker">Key Menu</p>
                <h2 id="key-editor-title">{selectedAssignment.label}</h2>
                <p className="section-copy">Set the action, RGB LED color, and brightness for this key.</p>
              </div>
              <button type="button" className="button button-secondary" onClick={handleCloseEditor} disabled={busy}>
                Close
              </button>
            </div>

            <div className="key-editor-preview" style={getColorStyle(draftColor, draftBrightness)}>
              <span className="selection-badge">Preview</span>
              <div className="key-editor-preview-row">
                <span className="key-led-preview key-led-preview-large" aria-hidden="true" />
                <div>
                  <strong>{draftColor}</strong>
                  <p>{formatBrightnessPercent(draftBrightness)} brightness · {getActionSummary(draftAction)}</p>
                </div>
              </div>
            </div>

            <div className="field-stack">
              <label className="field">
                <span>Action Type</span>
                <select
                  value={draftAction.type || ACTION_UNASSIGNED}
                  onChange={(event) => handleActionTypeChange(event.target.value)}
                >
                  {actionOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>

              {renderActionFields(draftAction, handleDraftFieldChange)}

              <div className="field-grid">
                <label className="field">
                  <span>LED Color</span>
                  <div className="color-picker-shell">
                    <input
                      className="color-picker-input"
                      type="color"
                      value={draftColor}
                      onChange={(event) => handleColorPickerChange(event.target.value)}
                    />
                    <div className="color-picker-copy">
                      <strong>{draftColor}</strong>
                      <p className="field-note">Uses the platform native wheel or square RGB picker.</p>
                    </div>
                  </div>
                </label>

                <label className="field">
                  <span>Hex Code</span>
                  <div className="hex-input-row">
                    <input
                      type="text"
                      value={draftColorInput}
                      onChange={(event) => handleColorInputChange(event.target.value)}
                      onBlur={handleColorInputBlur}
                      placeholder="#FF6B00"
                      maxLength={7}
                    />
                    <button type="button" className="button button-secondary" onClick={handleResetColor}>
                      Off
                    </button>
                  </div>
                </label>
              </div>

              <label className="field">
                <span>Brightness</span>
                <div className="brightness-row">
                  <input
                    className="brightness-slider"
                    type="range"
                    min={0}
                    max={BRIGHTNESS_MAX}
                    step={1}
                    value={draftBrightness}
                    onChange={(event) => handleBrightnessChange(Number(event.target.value))}
                  />
                  <div className="brightness-value">{formatBrightnessPercent(draftBrightness)}</div>
                </div>
                <p className="field-note">0% turns the LED off. 100% sends the stored color at full brightness.</p>
              </label>

              {showDeferredWiringNote(draftAction.type) ? (
                <p className="field-note">This action is stored now and can be wired to device execution later.</p>
              ) : null}
            </div>

            <div className="key-editor-actions">
              <button type="button" className="button button-secondary" onClick={handleClear} disabled={busy}>
                Clear action
              </button>
              <button type="button" className="button button-secondary" onClick={handleCloseEditor} disabled={busy}>
                Cancel
              </button>
              <button type="button" className="button button-primary" onClick={handleApply} disabled={busy || !draftDirty}>
                Apply
              </button>
            </div>
          </section>
        </div>
      ) : null}

      {loading ? <div className="loading-mask">Loading HPAD state...</div> : null}
    </div>
  );
}

function StatusChip(props: { title: string; status: string; tone: string }) {
  return (
    <div className={`status-chip ${toneClass(props.tone)}`}>
      <span className="status-chip-title">{props.title}</span>
      <strong>{props.status}</strong>
    </div>
  );
}

function StatusCard(props: { title: string; status: string; detail: string; tone: string }) {
  return (
    <article className={`status-card ${toneClass(props.tone)}`}>
      <div className="status-card-topline">
        <span>{props.title}</span>
        <strong>{props.status}</strong>
      </div>
      <p>{props.detail}</p>
    </article>
  );
}

function renderActionFields(
  action: KeyAction,
  onChange: (field: keyof KeyAction, value: string) => void,
) {
  switch (action.type || ACTION_UNASSIGNED) {
    case ACTION_KEYBOARD_SHORTCUT:
      return (
        <label className="field">
          <span>Shortcut</span>
          <input
            type="text"
            value={action.shortcut ?? ""}
            onChange={(event) => onChange("shortcut", event.target.value)}
            placeholder="Ctrl+Shift+P"
          />
        </label>
      );
    case ACTION_RUN_COMMAND:
      return (
        <>
          <label className="field">
            <span>Command</span>
            <input
              type="text"
              value={action.command ?? ""}
              onChange={(event) => onChange("command", event.target.value)}
              placeholder="/usr/bin/playerctl"
            />
          </label>
          <label className="field">
            <span>Arguments</span>
            <input
              type="text"
              value={action.arguments ?? ""}
              onChange={(event) => onChange("arguments", event.target.value)}
              placeholder="play-pause"
            />
          </label>
        </>
      );
    case ACTION_OPEN_APPLICATION:
      return (
        <label className="field">
          <span>Application</span>
          <input
            type="text"
            value={action.application ?? ""}
            onChange={(event) => onChange("application", event.target.value)}
            placeholder="/usr/bin/code"
          />
        </label>
      );
    case ACTION_MEDIA_CONTROL:
      return (
        <label className="field">
          <span>Media Control</span>
          <select
            value={action.mediaControl ?? mediaOptions[0].value}
            onChange={(event) => onChange("mediaControl", event.target.value)}
          >
            {mediaOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
      );
    default:
      return <p className="field-note">No action is assigned to this key.</p>;
  }
}

function createEmptyDashboardState() {
  return new DashboardState({
    profile: "Default",
    dongleStatus: { state: "not_detected", label: "Not Detected", detail: "USB HID dongle not detected" },
    macropadStatus: { state: "unknown", label: "Unknown", detail: "Waiting for dongle" },
    batteryStatus: { state: "waiting", label: "--.- V", detail: "Waiting for device report", batteryMV: 0, charging: false },
    keyAssignments: Array.from({ length: 6 }, (_, index) => createDefaultAssignment(index)),
    dirty: false,
    saved: true,
  });
}

function getBatteryChipStatus(batteryStatus: DashboardState["batteryStatus"]) {
  return batteryStatus.charging ? `${batteryStatus.label} USB` : batteryStatus.label;
}

function createDefaultAssignment(index: number) {
  return new KeyAssignment({
    id: `K${index + 1}`,
    label: `K${index + 1}`,
    color: DEFAULT_KEY_COLOR,
    brightness: DEFAULT_KEY_BRIGHTNESS,
    action: createEmptyAction(),
  });
}

function createEmptyAction() {
  return new KeyAction({ type: ACTION_UNASSIGNED });
}

function copyAction(action?: KeyAction) {
  return normalizeAction(
    new KeyAction({
      type: action?.type ?? ACTION_UNASSIGNED,
      shortcut: action?.shortcut ?? "",
      command: action?.command ?? "",
      arguments: action?.arguments ?? "",
      workingDirectory: action?.workingDirectory ?? "",
      application: action?.application ?? "",
      mediaControl: action?.mediaControl ?? mediaOptions[0].value,
    }),
  );
}

function getAssignmentColor(assignment?: KeyAssignment) {
  return normalizeColor(assignment?.color);
}

function getAssignmentBrightness(assignment?: KeyAssignment) {
  return normalizeBrightness(assignment?.brightness);
}

function getLEDSummary(assignment?: KeyAssignment) {
  return `${getAssignmentColor(assignment)} · ${formatBrightnessPercent(getAssignmentBrightness(assignment))}`;
}

function getColorStyle(color: string, brightness = DEFAULT_KEY_BRIGHTNESS): CSSProperties {
  return {
    "--key-led-color": normalizeColor(color),
    "--key-led-opacity": String(normalizeBrightness(brightness) / BRIGHTNESS_MAX),
  } as CSSProperties;
}

function sanitizeColorInput(value: string) {
  const digits = value.toUpperCase().replace(/[^0-9A-F]/g, "").slice(0, 6);
  return digits ? `#${digits}` : "";
}

function isValidColor(value: string) {
  return /^#[0-9A-F]{6}$/.test(value);
}

function normalizeColor(value?: string) {
  const next = sanitizeColorInput(value ?? "");
  return isValidColor(next) ? next : DEFAULT_KEY_COLOR;
}

function normalizeBrightness(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return DEFAULT_KEY_BRIGHTNESS;
  }
  return Math.min(BRIGHTNESS_MAX, Math.max(0, Math.round(value)));
}

function formatBrightnessPercent(value: number) {
  return `${Math.round((normalizeBrightness(value) / BRIGHTNESS_MAX) * 100)}%`;
}

function normalizeAction(action: KeyAction) {
  const next = new KeyAction({
    type: action.type || ACTION_UNASSIGNED,
    shortcut: action.shortcut ?? "",
    command: action.command ?? "",
    arguments: action.arguments ?? "",
    workingDirectory: action.workingDirectory ?? "",
    application: action.application ?? "",
    mediaControl: action.mediaControl ?? mediaOptions[0].value,
  });

  switch (next.type) {
    case ACTION_KEYBOARD_SHORTCUT:
      next.command = "";
      next.arguments = "";
      next.application = "";
      next.mediaControl = "";
      break;
    case ACTION_RUN_COMMAND:
      next.shortcut = "";
      next.application = "";
      next.mediaControl = "";
      break;
    case ACTION_OPEN_APPLICATION:
      next.shortcut = "";
      next.command = "";
      next.arguments = "";
      next.mediaControl = "";
      break;
    case ACTION_MEDIA_CONTROL:
      next.shortcut = "";
      next.command = "";
      next.arguments = "";
      next.application = "";
      if (!next.mediaControl) {
        next.mediaControl = mediaOptions[0].value;
      }
      break;
    default:
      return createEmptyAction();
  }

  return next;
}

function isAssigned(assignment: KeyAssignment) {
  return (assignment.action.type || ACTION_UNASSIGNED) !== ACTION_UNASSIGNED;
}

function getActionSummary(action: KeyAction) {
  switch (action.type || ACTION_UNASSIGNED) {
    case ACTION_KEYBOARD_SHORTCUT:
      return action.shortcut || "Shortcut not set";
    case ACTION_RUN_COMMAND:
      return [action.command, action.arguments].filter(Boolean).join(" ") || "Run command";
    case ACTION_OPEN_APPLICATION:
      return action.application || "Open application";
    case ACTION_MEDIA_CONTROL:
      return mediaOptions.find((option) => option.value === action.mediaControl)?.label || "Media control";
    default:
      return "Unassigned";
  }
}

function showDeferredWiringNote(actionType: string) {
  return actionType === ACTION_KEYBOARD_SHORTCUT || actionType === ACTION_MEDIA_CONTROL;
}

function toneClass(tone: string) {
  switch (tone) {
    case "connected":
    case "charging":
      return "tone-connected";
    case "disconnected":
    case "waiting":
      return "tone-waiting";
    case "not_detected":
      return "tone-alert";
    default:
      return "tone-neutral";
  }
}

function getErrorMessage(error: unknown) {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return "The HPAD backend request failed.";
}