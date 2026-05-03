import { KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

import { getActionSummary, isAssigned } from "../domain/action";
import { getAssignmentBrightness, getAssignmentColor, getColorStyle } from "../domain/color";

type KeyGridProps = {
  keyAssignments: KeyAssignment[];
  selectedIndex: number;
  onSelect: (index: number) => void;
};

export function KeyGrid(props: KeyGridProps) {
  return (
    <div className="pad-preview" aria-label="HPAD key preview">
      <div className="pad-preview-grid" role="grid">
        {props.keyAssignments.map((assignment: KeyAssignment, index: number) => {
          const assigned = isAssigned(assignment);
          const summary = getActionSummary(assignment.action);

          return (
            <button
              key={assignment.id || index}
              type="button"
              className={`pad-key ${index === props.selectedIndex ? "selected" : ""} ${assigned ? "assigned" : "empty"}`}
              onClick={() => props.onSelect(index)}
              aria-label={assigned ? `${assignment.label}. Assigned. ${summary}.` : `${assignment.label}. Unassigned.`}
              aria-pressed={index === props.selectedIndex}
              style={getColorStyle(getAssignmentColor(assignment), getAssignmentBrightness(assignment))}
              title={`${assignment.label}: ${summary}`}
            >
              <span className="pad-key-label">{assignment.label}</span>
              <span className="led-swatch pad-key-led" aria-hidden="true" />
            </button>
          );
        })}
      </div>
    </div>
  );
}