import { useEffect, useState } from "react";

import {
  ApplyKeyAction,
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
  const [draftAction, setDraftAction] = useState<KeyAction>(() => createEmptyAction());
  const [draftDirty, setDraftDirty] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

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
          setDraftAction(copyAction(next.keyAssignments[selectedIndex]?.action));
          setDraftDirty(false);
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
    setDraftAction(copyAction(dashboard.keyAssignments[index]?.action));
    setDraftDirty(false);
    setErrorMessage(null);
  };

  const handleActionTypeChange = (nextType: string) => {
    setDraftAction((current) => normalizeAction({ ...current, type: nextType }));
    setDraftDirty(true);
  };

  const handleDraftFieldChange = (field: keyof KeyAction, value: string) => {
    setDraftAction((current) => ({ ...current, [field]: value }));
    setDraftDirty(true);
  };

  const handleApply = async () => {
    setBusy(true);
    try {
      const next = await ApplyKeyAction(selectedIndex, normalizeAction(draftAction));
      setDashboard(next);
      setDraftAction(copyAction(next.keyAssignments[selectedIndex]?.action));
      setDraftDirty(false);
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
      setDraftAction(copyAction(next.keyAssignments[selectedIndex]?.action));
      setDraftDirty(false);
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
              <p className="section-copy">Select a key to edit its assignment in the inspector.</p>
            </div>
            <div className="key-grid">
              {dashboard.keyAssignments.map((assignment, index) => (
                <button
                  key={assignment.id || index}
                  type="button"
                  className={`key-card ${index === selectedIndex ? "selected" : ""} ${isAssigned(assignment) ? "assigned" : "empty"}`}
                  onClick={() => handleSelectKey(index)}
                >
                  <div className="key-card-header">
                    <span className="key-label">{assignment.label}</span>
                    <span className="key-state">{isAssigned(assignment) ? "Assigned" : "Open"}</span>
                  </div>
                  <p className="key-summary">{getActionSummary(assignment.action)}</p>
                </button>
              ))}
            </div>
          </section>
        </div>

        <aside className="panel inspector-panel">
          <div className="section-header inspector-header">
            <div>
              <p className="section-kicker">Inspector</p>
              <h2>{selectedAssignment.label}</h2>
            </div>
            <span className="selection-badge">Selected key</span>
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

            {showDeferredWiringNote(draftAction.type) ? (
              <p className="field-note">This action is stored now and can be wired to device execution later.</p>
            ) : null}
          </div>

          <div className="inspector-actions">
            <button type="button" className="button button-primary" onClick={handleApply} disabled={busy}>
              Apply
            </button>
            <button type="button" className="button button-secondary" onClick={handleClear} disabled={busy}>
              Clear
            </button>
          </div>
        </aside>
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