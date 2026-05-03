import { KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

import { getActionSummary, isAssigned } from "../domain/action";
import { getAssignmentBrightness, getAssignmentColor, getColorStyle, getLEDSummary } from "../domain/color";

type KeyGridProps = {
  keyAssignments: KeyAssignment[];
  selectedIndex: number;
  onSelect: (index: number) => void;
};

export function KeyGrid(props: KeyGridProps) {
  return (
    <div className="key-grid" role="grid" aria-label="HPAD layout">
      {props.keyAssignments.map((assignment: KeyAssignment, index: number) => (
        <button
          key={assignment.id || index}
          type="button"
          className={`key-card ${index === props.selectedIndex ? "selected" : ""} ${isAssigned(assignment) ? "assigned" : "empty"}`}
          onClick={() => props.onSelect(index)}
          aria-pressed={index === props.selectedIndex}
          style={getColorStyle(getAssignmentColor(assignment), getAssignmentBrightness(assignment))}
        >
          <div className="key-card-header">
            <span className="key-label">{assignment.label}</span>
            <span className={`key-state ${isAssigned(assignment) ? "is-assigned" : ""}`}>{isAssigned(assignment) ? "Assigned" : "Empty"}</span>
          </div>
          <div className="key-led-row">
            <span className="key-led-preview" aria-hidden="true" />
            <span className="key-led-value">{getLEDSummary(assignment)}</span>
          </div>
          <p className="key-summary">{getActionSummary(assignment.action)}</p>
        </button>
      ))}
    </div>
  );
}