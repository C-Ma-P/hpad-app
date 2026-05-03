import { type CSSProperties } from "react";

import { KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

export const DEFAULT_KEY_COLOR = "#000000";
export const DEFAULT_KEY_BRIGHTNESS = 0xff;
export const BRIGHTNESS_MAX = 0xff;

export function getAssignmentColor(assignment?: KeyAssignment) {
  return normalizeColor(assignment?.color);
}

export function getAssignmentBrightness(assignment?: KeyAssignment) {
  return normalizeBrightness(assignment?.brightness);
}

export function getLEDSummary(assignment?: KeyAssignment) {
  return `${getAssignmentColor(assignment)} · ${formatBrightnessPercent(getAssignmentBrightness(assignment))}`;
}

export function getColorStyle(color: string, brightness = DEFAULT_KEY_BRIGHTNESS): CSSProperties {
  return {
    "--key-led-color": normalizeColor(color),
    "--key-led-opacity": String(normalizeBrightness(brightness) / BRIGHTNESS_MAX),
  } as CSSProperties;
}

export function sanitizeColorInput(value: string) {
  const digits = value.toUpperCase().replace(/[^0-9A-F]/g, "").slice(0, 6);
  return digits ? `#${digits}` : "";
}

export function isValidColor(value: string) {
  return /^#[0-9A-F]{6}$/.test(value);
}

export function normalizeColor(value?: string) {
  const next = sanitizeColorInput(value ?? "");
  return isValidColor(next) ? next : DEFAULT_KEY_COLOR;
}

export function normalizeBrightness(value?: number | null) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return DEFAULT_KEY_BRIGHTNESS;
  }
  return Math.min(BRIGHTNESS_MAX, Math.max(0, Math.round(value)));
}

export function formatBrightnessPercent(value: number) {
  return `${Math.round((normalizeBrightness(value) / BRIGHTNESS_MAX) * 100)}%`;
}