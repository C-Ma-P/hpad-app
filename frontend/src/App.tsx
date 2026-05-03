import { useEffect, useState } from "react";

import {
  ApplyKeySettings,
  ClearKeyAction,
  GetDashboardState,
  SaveToDevice,
} from "../bindings/hpad-app/internal/agentservice/agentservice.js";
import { DashboardState } from "../bindings/hpad-app/internal/app/models.js";
import { KeyAction, KeyAssignment } from "../bindings/hpad-app/internal/config/models.js";

import { KeyInspector } from "./components/KeyInspector";
import { KeyGrid } from "./components/KeyGrid";
import { StatusChip } from "./components/StatusCard";
import {
  copyAction,
  createEmptyAction,
  isAssigned,
  normalizeAction,
} from "./domain/action";
import {
  DEFAULT_KEY_BRIGHTNESS,
  DEFAULT_KEY_COLOR,
  getAssignmentBrightness,
  getAssignmentColor,
  isValidColor,
  normalizeBrightness,
  normalizeColor,
  sanitizeColorInput,
} from "./domain/color";

const STATUS_REFRESH_MS = 2500;

export default function App() {
  const [dashboard, setDashboard] = useState<DashboardState>(() => createEmptyDashboardState());
  const [selectedIndex, setSelectedIndex] = useState(0);
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
  const assignedCount = dashboard.keyAssignments.filter(isAssigned).length;
  const configStateLabel = dashboard.dirty ? "Dirty" : "Saved";
  const saveButtonLabel = busy ? "Saving..." : "Save to Device";

  const handleSelectKey = (index: number) => {
    setSelectedIndex(index);
    hydrateDraft(dashboard.keyAssignments[index]);
    setErrorMessage(null);
  };

  const handleRevert = () => {
    if (busy) {
      return;
    }
    hydrateDraft(dashboard.keyAssignments[selectedIndex]);
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
      <header className="topbar panel">
        <div className="toolbar-brand">
          <span className="app-title">HPAD</span>
          <div className="toolbar-profile">
            <span className="toolbar-label">Profile</span>
            <strong>{dashboard.profile}</strong>
          </div>
        </div>
        <div className="toolbar-strip">
          <StatusChip title="Dongle" status={dashboard.dongleStatus.label} tone={dashboard.dongleStatus.state} />
          <StatusChip title="Macropad" status={dashboard.macropadStatus.label} tone={dashboard.macropadStatus.state} />
          <StatusChip title="Power" status={getBatteryChipStatus(dashboard.batteryStatus)} tone={dashboard.batteryStatus.state} />
          <div className={`toolbar-state ${dashboard.dirty ? "dirty" : "saved"}`}>
            <span className="toolbar-label">Config</span>
            <strong>{configStateLabel}</strong>
          </div>
        </div>
      </header>

      <main className="workspace">
        <section className="panel section-panel layout-panel">
          <div className="section-header layout-header">
            <div>
              <p className="section-kicker">Device</p>
              <h2>HPAD Layout</h2>
            </div>
            <div className="layout-meta">
              <span>{assignedCount}/6 assigned</span>
              <span>{selectedAssignment.label} selected</span>
            </div>
          </div>
          <p className="section-copy">Select a key on the pad to inspect its action and LED settings.</p>

          <div className="layout-canvas">
            <div className="device-shell">
              <KeyGrid keyAssignments={dashboard.keyAssignments} selectedIndex={selectedIndex} onSelect={handleSelectKey} />
            </div>
          </div>
        </section>

        <KeyInspector
          assignment={selectedAssignment}
          draftAction={draftAction}
          draftColor={draftColor}
          draftColorInput={draftColorInput}
          draftBrightness={draftBrightness}
          draftDirty={draftDirty}
          busy={busy}
          onActionTypeChange={handleActionTypeChange}
          onDraftFieldChange={handleDraftFieldChange}
          onColorPickerChange={handleColorPickerChange}
          onColorInputChange={handleColorInputChange}
          onColorInputBlur={handleColorInputBlur}
          onResetColor={handleResetColor}
          onBrightnessChange={handleBrightnessChange}
          onClear={handleClear}
          onRevert={handleRevert}
          onApply={handleApply}
        />
      </main>

      <footer className="footerbar panel">
        <div className="footer-status-line">
          <span className={`footer-state ${dashboard.dirty ? "dirty" : "saved"}`}>{configStateLabel}</span>
          <span className="footer-copy">
            {dashboard.dirty ? "Changes pending save to device." : "Configuration synced to device."}
          </span>
          {errorMessage ? (
            <span className="footer-error">{errorMessage}</span>
          ) : (
            <span className="footer-hint">
              {draftDirty ? `${selectedAssignment.label} has unapplied inspector changes.` : "No recent errors."}
            </span>
          )}
        </div>
        <button type="button" className="button button-primary footer-button" onClick={handleSave} disabled={busy || !dashboard.dirty}>
          {saveButtonLabel}
        </button>
      </footer>

      {loading ? <div className="loading-mask">Loading HPAD state...</div> : null}
    </div>
  );
}

function createEmptyDashboardState() {
  return new DashboardState({
    profile: "Default",
    dongleStatus: { state: "not_detected", label: "Not Detected", detail: "USB HID dongle not detected" },
    macropadStatus: { state: "unknown", label: "Unknown", detail: "Waiting for dongle" },
    batteryStatus: { state: "waiting", label: "--.- V", detail: "Waiting for device report", batteryMV: 0, usbPowerPresent: false },
    keyAssignments: Array.from({ length: 6 }, (_, index) => createDefaultAssignment(index)),
    dirty: false,
    saved: true,
  });
}

function getBatteryChipStatus(batteryStatus: DashboardState["batteryStatus"]) {
  return batteryStatus.usbPowerPresent ? `${batteryStatus.label} USB` : batteryStatus.label;
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

function getErrorMessage(error: unknown) {
  const rawMessage = error instanceof Error ? error.message : typeof error === "string" ? error : "";

  if (!rawMessage) {
    return "The HPAD backend request failed.";
  }

  if (rawMessage.includes("Unsupported method ('POST')") || rawMessage.includes("/wails/runtime")) {
    return "Backend unavailable in browser preview.";
  }

  const htmlTitle = rawMessage.match(/<h1>([^<]+)<\/h1>/i)?.[1];
  const htmlMessage = rawMessage.match(/<p>Message:\s*([^<]+)<\/p>/i)?.[1];
  if (htmlTitle || htmlMessage) {
    return [htmlTitle, htmlMessage].filter(Boolean).join(": ");
  }

  const normalized = rawMessage.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();
  return normalized || "The HPAD backend request failed.";
}