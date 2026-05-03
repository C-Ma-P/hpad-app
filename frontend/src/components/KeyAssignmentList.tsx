import { KeyAssignment } from "../../bindings/hpad-app/internal/config/models.js";

import { getActionSummary, isAssigned } from "../domain/action";
import { getAssignmentBrightness, getAssignmentColor, getColorStyle } from "../domain/color";

type KeyAssignmentListProps = {
  keyAssignments: KeyAssignment[];
  selectedIndex: number;
  onSelect: (index: number) => void;
};

export function KeyAssignmentList(props: KeyAssignmentListProps) {
  return (
    <div className="assignment-table" aria-label="Key assignments">
      <div className="assignment-row assignment-header" aria-hidden="true">
        <span className="assignment-cell assignment-cell-key">Key</span>
        <span className="assignment-cell assignment-cell-action">Action</span>
        <span className="assignment-cell assignment-cell-led">LED</span>
      </div>

      {props.keyAssignments.map((assignment: KeyAssignment, index: number) => {
        const assigned = isAssigned(assignment);
        const summary = getActionSummary(assignment.action);

        return (
          <button
            key={assignment.id || index}
            type="button"
            className={`assignment-row ${index === props.selectedIndex ? "selected" : ""} ${assigned ? "assigned" : "empty"}`}
            onClick={() => props.onSelect(index)}
            aria-label={`Select ${assignment.label}: ${summary}`}
            aria-pressed={index === props.selectedIndex}
            title={`${assignment.label}: ${summary}`}
          >
            <span className="assignment-cell assignment-cell-key">{assignment.label}</span>
            <span className="assignment-cell assignment-cell-action">{summary}</span>
            <span className="assignment-cell assignment-cell-led">
              <span
                className="led-swatch"
                aria-hidden="true"
                style={getColorStyle(getAssignmentColor(assignment), getAssignmentBrightness(assignment))}
              />
            </span>
          </button>
        );
      })}
    </div>
  );
}