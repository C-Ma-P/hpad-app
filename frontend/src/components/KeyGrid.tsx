import { KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

import { getActionSummary, isAssigned } from "../domain/action";
import { getAssignmentColor, getColorStyle, getLEDSummary } from "../domain/color";

type KeyGridProps = {
  keyAssignments: KeyAssignment[];
  editorOpen: boolean;
  selectedIndex: number;
  onSelect: (index: number) => void;
};

export function KeyGrid(props: KeyGridProps) {
  return (
    <div className="key-grid">
      {props.keyAssignments.map((assignment: KeyAssignment, index: number) => (
        <button
          key={assignment.id || index}
          type="button"
          className={`key-card ${props.editorOpen && index === props.selectedIndex ? "selected" : ""} ${isAssigned(assignment) ? "assigned" : "empty"}`}
          onClick={() => props.onSelect(index)}
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
  );
}