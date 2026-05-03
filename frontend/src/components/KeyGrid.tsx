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
    <div className="key-grid" role="grid" aria-label="HPAD device layout">
      {props.keyAssignments.map((assignment: KeyAssignment, index: number) => {
        const assigned = isAssigned(assignment);
        const summary = getActionSummary(assignment.action);

        return (
          <button
            key={assignment.id || index}
            type="button"
            className={`key-card ${index === props.selectedIndex ? "selected" : ""} ${assigned ? "assigned" : "empty"}`}
            onClick={() => props.onSelect(index)}
            aria-label={assigned ? `${assignment.label}. Assigned. ${summary}.` : `${assignment.label}. Unassigned.`}
            aria-pressed={index === props.selectedIndex}
            style={getColorStyle(getAssignmentColor(assignment), getAssignmentBrightness(assignment))}
            title={`${assignment.label}: ${summary}`}
          >
            <div className="key-card-header">
              <span className="key-label">{assignment.label}</span>
            </div>
            <p className="key-summary">{summary}</p>
          </button>
        );
      })}
    </div>
  );
}