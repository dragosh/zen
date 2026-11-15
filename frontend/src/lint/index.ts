// ui/lint: Vale alerts in the reader (markText) and the editor (backdrop),
// with a count button and panel (renderPanel) in both.
import "./lint.css";

export { clearMarks, flash, markText, severityOf, titleOf } from "./mark";
export { renderPanel, type LintPanel } from "./panel";
export { alertRange, backdrop, type Backdrop } from "./backdrop";
