import { KeyAction, KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

export const ACTION_UNASSIGNED = "unassigned";
export const ACTION_KEYBOARD_SHORTCUT = "keyboard_shortcut";
export const ACTION_RUN_COMMAND = "run_command";
export const ACTION_OPEN_APPLICATION = "open_application";
export const ACTION_MEDIA_CONTROL = "media_control";

export const actionOptions = [
  { value: ACTION_UNASSIGNED, label: "Unassigned" },
  { value: ACTION_KEYBOARD_SHORTCUT, label: "Keyboard shortcut" },
  { value: ACTION_RUN_COMMAND, label: "Run command" },
  { value: ACTION_OPEN_APPLICATION, label: "Open application" },
  { value: ACTION_MEDIA_CONTROL, label: "Media control" },
];

export const mediaOptions = [
  { value: "play_pause", label: "Play / Pause" },
  { value: "next_track", label: "Next track" },
  { value: "previous_track", label: "Previous track" },
  { value: "volume_up", label: "Volume up" },
  { value: "volume_down", label: "Volume down" },
  { value: "mute", label: "Mute" },
];

export function createEmptyAction() {
  return new KeyAction({ type: ACTION_UNASSIGNED });
}

export function copyAction(action?: KeyAction) {
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

export function normalizeAction(action: KeyAction) {
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

export function isAssigned(assignment: KeyAssignment) {
  return (assignment.action.type || ACTION_UNASSIGNED) !== ACTION_UNASSIGNED;
}

export function getActionSummary(action: KeyAction) {
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

export function showDeferredWiringNote(actionType: string | undefined) {
  return actionType === ACTION_KEYBOARD_SHORTCUT || actionType === ACTION_MEDIA_CONTROL;
}