import { KeyAction, KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

import {
  ACTION_KEYBOARD_SHORTCUT,
  ACTION_MEDIA_CONTROL,
  ACTION_OPEN_APPLICATION,
  ACTION_RUN_COMMAND,
  ACTION_UNASSIGNED,
  actionOptions,
  getActionSummary,
  isAssigned,
  mediaOptions,
  showDeferredWiringNote,
} from "../domain/action";
import { BRIGHTNESS_MAX, formatBrightnessPercent, getColorStyle } from "../domain/color";

type KeyInspectorProps = {
  assignment: KeyAssignment;
  draftAction: KeyAction;
  draftColor: string;
  draftColorInput: string;
  draftBrightness: number;
  draftDirty: boolean;
  busy: boolean;
  onActionTypeChange: (nextType: string) => void;
  onDraftFieldChange: (field: keyof KeyAction, value: string) => void;
  onColorPickerChange: (value: string) => void;
  onColorInputChange: (value: string) => void;
  onColorInputBlur: () => void;
  onResetColor: () => void;
  onBrightnessChange: (value: number) => void;
  onClear: () => void;
  onRevert: () => void;
  onApply: () => void;
};

export function KeyInspector(props: KeyInspectorProps) {
  const assignmentState = isAssigned(props.assignment) ? "Assigned" : "Unassigned";

  return (
    <aside className="panel inspector-panel" aria-labelledby="key-inspector-title">
      <div className="inspector-header">
        <div>
          <p className="section-kicker">Inspector</p>
          <h2 id="key-inspector-title">{props.assignment.label}</h2>
        </div>
        <span className={`selection-badge ${isAssigned(props.assignment) ? "is-assigned" : ""}`}>{assignmentState}</span>
      </div>

      <div className="inspector-meta">
        <div className="inspector-meta-row">
          <span>Stored</span>
          <strong>{getActionSummary(props.assignment.action)}</strong>
        </div>
        <div className="inspector-meta-row">
          <span>Draft</span>
          <strong>{props.draftDirty ? "Pending changes" : "Matches stored state"}</strong>
        </div>
      </div>

      <section className="inspector-section">
        <p className="inspector-section-title">Action</p>
        <div className="field-stack">
          <label className="field">
            <span>Action Type</span>
            <select
              value={props.draftAction.type || ACTION_UNASSIGNED}
              onChange={(event) => props.onActionTypeChange(event.target.value)}
            >
              {actionOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>

          {renderActionFields(props.draftAction, props.onDraftFieldChange)}

          {showDeferredWiringNote(props.draftAction.type) ? (
            <p className="field-note">This action is stored now and can be wired to device execution later.</p>
          ) : null}
        </div>
      </section>

      <section className="inspector-section">
        <p className="inspector-section-title">Lighting</p>
        <div className="field-stack">
          <div className="field-grid">
            <label className="field">
              <span>LED Color</span>
              <div className="color-picker-shell">
                <input
                  className="color-picker-input"
                  type="color"
                  value={props.draftColor}
                  onChange={(event) => props.onColorPickerChange(event.target.value)}
                />
                <div className="color-picker-copy">
                  <strong>{props.draftColor}</strong>
                  <p className="field-note">Uses the native platform color picker.</p>
                </div>
              </div>
            </label>

            <label className="field">
              <span>Hex Code</span>
              <div className="hex-input-row">
                <input
                  type="text"
                  value={props.draftColorInput}
                  onChange={(event) => props.onColorInputChange(event.target.value)}
                  onBlur={props.onColorInputBlur}
                  placeholder="#FF6B00"
                  maxLength={7}
                />
                <button type="button" className="button button-secondary" onClick={props.onResetColor} disabled={props.busy}>
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
                value={props.draftBrightness}
                onChange={(event) => props.onBrightnessChange(Number(event.target.value))}
              />
              <div className="brightness-value">{formatBrightnessPercent(props.draftBrightness)}</div>
            </div>
            <p className="field-note">0% turns the LED off. 100% sends the stored color at full brightness.</p>
          </label>
        </div>
      </section>

      <section className="inspector-section">
        <p className="inspector-section-title">Preview</p>
        <div className="inspector-preview-row" style={getColorStyle(props.draftColor, props.draftBrightness)}>
          <span className="key-led-preview inspector-led-swatch" aria-hidden="true" />
          <div className="inspector-preview-copy">
            <strong>{props.draftColor}</strong>
            <p>
              {formatBrightnessPercent(props.draftBrightness)} brightness · {getActionSummary(props.draftAction)}
            </p>
          </div>
        </div>
      </section>

      <div className="inspector-actions">
        <button type="button" className="button button-secondary" onClick={props.onClear} disabled={props.busy}>
          Clear Action
        </button>
        <button type="button" className="button button-secondary" onClick={props.onRevert} disabled={props.busy || !props.draftDirty}>
          Revert
        </button>
        <button type="button" className="button button-primary" onClick={props.onApply} disabled={props.busy || !props.draftDirty}>
          Apply
        </button>
      </div>
    </aside>
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